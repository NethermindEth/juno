package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const (
	// wsReadLimit caps the size of a single websocket message. 16 MiB
	// is generous for an Ethereum log payload — block gas limits make
	// real logs far smaller — but still bounds an adversarial sender.
	wsReadLimit = 16 << 20

	// wsLogSubBuffer is the per-subscription pending-notification
	// buffer. Sized so a stuck consumer doesn't immediately stall the
	// reader, but bounded so an idle subscription doesn't pin memory.
	wsLogSubBuffer = 64

	// wsUnsubscribeTimeout is how long Unsubscribe waits for the server
	// to acknowledge eth_unsubscribe before giving up.
	wsUnsubscribeTimeout = 2 * time.Second
)

// rpcReply is the message exchanged on a pending-call channel: either
// a result payload or a server-side error.
type rpcReply struct {
	result json.RawMessage
	err    error
}

// wsTransport speaks JSON-RPC 2.0 over a single persistent websocket
// connection. Both unary calls and eth_subscribe notifications travel
// over the same conn and are routed by request id / subscription id.
type wsTransport struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	nextID  atomic.Uint64

	mu      sync.Mutex
	pending map[uint64]chan rpcReply // by request id
	// pendingSubs: subscribe calls awaiting the server-assigned sub id, keyed by request id.
	pendingSubs map[uint64]*wsLogSub
	subs        map[string]*wsLogSub // active subscriptions, by sub id

	closed    chan struct{}
	closeErr  error
	closeOnce sync.Once
}

// dialWS dials rawURL and starts the reader goroutine. The caller is
// responsible for calling close to release the connection.
func dialWS(ctx context.Context, rawURL string) (*wsTransport, error) {
	conn, resp, err := websocket.Dial(ctx, rawURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("dial ws: %w", err)
	}
	conn.SetReadLimit(wsReadLimit)
	t := &wsTransport{
		conn:        conn,
		pending:     make(map[uint64]chan rpcReply),
		pendingSubs: make(map[uint64]*wsLogSub),
		subs:        make(map[string]*wsLogSub),
		closed:      make(chan struct{}),
	}
	go t.readLoop() //nolint:gosec // G118: long-lived loop, not request-scoped
	return t, nil
}

// readLoop drains incoming frames until the connection breaks. Each
// frame is dispatched as either a response (id-matched) or a
// subscription notification (method == "eth_subscription"). Anything
// malformed is dropped: a misbehaving remote manifests as a call
// timeout, which is the right user-visible signal.
func (t *wsTransport) readLoop() {
	for {
		_, data, err := t.conn.Read(context.Background())
		if err != nil {
			t.shutdown(err)
			return
		}
		t.dispatch(data)
	}
}

func (t *wsTransport) dispatch(data []byte) {
	var probe struct {
		ID     json.RawMessage `json:"id,omitempty"`
		Method string          `json:"method,omitempty"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return
	}
	switch {
	case probe.Method == "eth_subscription":
		t.dispatchNotification(data)
	case len(probe.ID) > 0 && !bytesEqual(probe.ID, jsonNull):
		t.dispatchResponse(data)
	}
}

func (t *wsTransport) dispatchResponse(data []byte) {
	var resp rpcResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}

	t.mu.Lock()
	ch, hasPending := t.pending[resp.ID]
	delete(t.pending, resp.ID)
	pendingSub, isSubscribe := t.pendingSubs[resp.ID]
	delete(t.pendingSubs, resp.ID)
	t.mu.Unlock()

	if !hasPending {
		return
	}

	reply := rpcReply{}
	switch {
	case resp.Error != nil:
		reply.err = resp.Error
	case isSubscribe:
		// Decode the subscription id and register the sub BEFORE the
		// caller's goroutine wakes up. Otherwise a notification could
		// race in (the reader processes one frame at a time, but the
		// caller doesn't get scheduled in lockstep).
		var subID string
		if err := json.Unmarshal(resp.Result, &subID); err != nil {
			reply.err = fmt.Errorf("decode subscription id: %w", err)
		} else if subID == "" {
			reply.err = errors.New("empty subscription id")
		} else {
			pendingSub.id = subID
			t.mu.Lock()
			if t.subs != nil {
				t.subs[subID] = pendingSub
				reply.result = resp.Result
			} else {
				reply.err = ErrTransportClosed
			}
			t.mu.Unlock()
		}
	default:
		reply.result = resp.Result
	}

	// Best-effort send: ch is buffered to 1 so this can't block. If the
	// caller already gave up (ctx cancelled), no one will read it.
	select {
	case ch <- reply:
	default:
	}
}

func (t *wsTransport) dispatchNotification(data []byte) {
	var notif struct {
		Method string `json:"method"`
		Params struct {
			Subscription string          `json:"subscription"`
			Result       json.RawMessage `json:"result"`
		} `json:"params"`
	}
	if err := json.Unmarshal(data, &notif); err != nil {
		return
	}
	t.mu.Lock()
	sub := t.subs[notif.Params.Subscription]
	t.mu.Unlock()
	if sub == nil {
		return
	}
	select {
	case sub.logCh <- notif.Params.Result:
	case <-sub.closed:
	case <-t.closed:
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// shutdown is the single termination path. It fans the cause out to
// every pending caller and active subscription, then closes the conn.
func (t *wsTransport) shutdown(cause error) {
	t.closeOnce.Do(func() {
		if cause == nil {
			cause = ErrTransportClosed
		}
		t.mu.Lock()
		pending, pendingSubs, subs := t.pending, t.pendingSubs, t.subs
		t.pending = nil
		t.pendingSubs = nil
		t.subs = nil
		t.closeErr = cause
		t.mu.Unlock()
		close(t.closed)

		for _, ch := range pending {
			select {
			case ch <- rpcReply{err: cause}:
			default:
			}
		}
		for _, sub := range pendingSubs {
			sub.fail(cause)
		}
		for _, sub := range subs {
			sub.fail(cause)
		}
		// Use CloseNow so we don't block on a handshake the remote may
		// already have abandoned.
		_ = t.conn.CloseNow()
	})
}

func (t *wsTransport) close() { t.shutdown(ErrTransportClosed) }

// writeJSON serialises and sends one frame. Writes are serialised
// because coder/websocket's Write is not concurrency-safe for arbitrary
// callers (only one Writer/Reader pair may be active at a time).
func (t *wsTransport) writeJSON(ctx context.Context, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return t.conn.Write(ctx, websocket.MessageText, data)
}

// call sends a JSON-RPC request and waits for its response.
func (t *wsTransport) call(
	ctx context.Context,
	method string,
	params ...any,
) (json.RawMessage, error) {
	return t.callWithSubReg(ctx, method, nil, params...)
}

// callWithSubReg is the shared implementation for unary calls and
// subscribe calls. When pendingSub is non-nil, the response handler
// extracts the subscription id and registers the sub atomically with
// the reply delivery.
func (t *wsTransport) callWithSubReg(
	ctx context.Context, method string, pendingSub *wsLogSub, params ...any,
) (json.RawMessage, error) {
	if params == nil {
		params = []any{}
	}
	id := t.nextID.Add(1)
	ch := make(chan rpcReply, 1)

	t.mu.Lock()
	if t.pending == nil {
		t.mu.Unlock()
		return nil, fmt.Errorf("%s: %w", method, ErrTransportClosed)
	}
	t.pending[id] = ch
	if pendingSub != nil {
		t.pendingSubs[id] = pendingSub
	}
	t.mu.Unlock()

	deregister := func() {
		t.mu.Lock()
		if t.pending != nil {
			delete(t.pending, id)
			delete(t.pendingSubs, id)
		}
		t.mu.Unlock()
	}

	if err := t.writeJSON(ctx, rpcRequest{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}); err != nil {
		deregister()
		return nil, fmt.Errorf("%s: write: %w", method, err)
	}

	select {
	case reply := <-ch:
		// dispatchResponse already removed our entry; deregister is a no-op.
		if reply.err != nil {
			return nil, reply.err
		}
		return reply.result, nil
	case <-ctx.Done():
		deregister()
		return nil, ctx.Err()
	case <-t.closed:
		deregister()
		return nil, fmt.Errorf("%s: %w", method, t.closeErr)
	}
}
