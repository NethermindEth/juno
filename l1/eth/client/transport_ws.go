package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/utils/log"
	"github.com/coder/websocket"
	"go.uber.org/zap"
)

const (
	// wsReadLimit caps the size of a single websocket message. 16 MiB
	// is generous for an Ethereum log payload — block gas limits make
	// real logs far smaller — but still bounds an adversarial sender.
	wsReadLimit = 16 << 20

	// wsLogSubBuffer is the per-subscription pending-notification
	// buffer. Sized so a stuck consumer doesn't immediately stall the
	// reader, but bounded so an idle subscription doesn't pin memory.
	//
	// Liveness invariant: dispatchNotification blocks the single
	// readLoop goroutine on logCh until the buffer drains, the sub
	// closes, or the transport closes. While blocked, no responses to
	// in-flight unary calls are dispatched and no control frames are
	// processed — meaning a stalled subscription consumer stalls every
	// unary RPC on the shared conn and eventually trips the ping
	// timeout. The 64-deep buffer plus infrequent log cadence keep this
	// safe in practice, but callers MUST drain their subscription sinks
	// promptly; a sink read on a long ticker (e.g. minute-scale) only
	// works because the buffer absorbs the gap.
	wsLogSubBuffer = 64

	// wsUnsubscribeTimeout is how long Unsubscribe waits for the server
	// to acknowledge eth_unsubscribe before giving up.
	wsUnsubscribeTimeout = 2 * time.Second

	// wsPingInterval is how long the connection may sit idle before we
	// send a keep-alive ping. Matches go-ethereum's rpc/websocket.go
	// constant — that value has held up against every major hosted RPC
	// provider (Alchemy, Infura, QuickNode) and any Cloudflare-class
	// proxy in front of them.
	wsPingInterval = 30 * time.Second

	// wsPingTimeout bounds a single ping round-trip. A wedged write or
	// stalled pong reply trips this and tears the transport down via
	// the same path as a read error, instead of letting the reader
	// silently hang.
	wsPingTimeout = 10 * time.Second
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
	logger  log.StructuredLogger

	mu      sync.Mutex
	pending map[uint64]chan rpcReply // by request id
	// pendingSubs: subscribe calls awaiting the server-assigned sub id, keyed by request id.
	pendingSubs map[uint64]*wsLogSub
	subs        map[string]*wsLogSub // active subscriptions, by sub id

	// pingReset is signalled (best-effort) after every successful
	// writeJSON so pingLoop can defer the next idle ping. Buffered to 1
	// so writers never block on a busy reset.
	pingReset    chan struct{}
	pingInterval time.Duration
	pingTimeout  time.Duration

	closed    chan struct{}
	closeErr  error
	closeOnce sync.Once
}

// dialWS dials rawURL and starts the reader and ping goroutines. The
// caller is responsible for calling close to release the connection.
// A zero opts.logger is replaced with a no-op logger; zero ping
// durations fall back to the package defaults.
func dialWS(ctx context.Context, rawURL string, opts options) (*wsTransport, error) {
	if opts.logger == nil {
		opts.logger = log.NewNopZapLogger()
	}
	if opts.pingInterval <= 0 {
		opts.pingInterval = wsPingInterval
	}
	if opts.pingTimeout <= 0 {
		opts.pingTimeout = wsPingTimeout
	}
	conn, resp, err := websocket.Dial(ctx, rawURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("dial ws: %w", err)
	}
	conn.SetReadLimit(wsReadLimit)
	t := &wsTransport{
		conn:         conn,
		logger:       opts.logger,
		pending:      make(map[uint64]chan rpcReply),
		pendingSubs:  make(map[uint64]*wsLogSub),
		subs:         make(map[string]*wsLogSub),
		pingReset:    make(chan struct{}, 1),
		pingInterval: opts.pingInterval,
		pingTimeout:  opts.pingTimeout,
		closed:       make(chan struct{}),
	}
	go t.readLoop() //nolint:gosec // G118: long-lived loop, not request-scoped
	go t.pingLoop() //nolint:gosec // G118: long-lived loop, not request-scoped
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

// pingLoop sends a keep-alive ping after pingInterval of silence on the
// connection. Any outbound write resets the timer via pingReset, so a
// busy connection issues no redundant pings. A ping failure (write
// stall, pong timeout, transport already torn) goes through the same
// shutdown path as a read error — the redial layer above handles the
// rest.
func (t *wsTransport) pingLoop() {
	timer := time.NewTimer(t.pingInterval)
	defer timer.Stop()
	for {
		select {
		case <-t.closed:
			return
		case <-t.pingReset:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(t.pingInterval)
		case <-timer.C:
			ctx, cancel := context.WithTimeout(context.Background(), t.pingTimeout)
			err := t.conn.Ping(ctx)
			cancel()
			if err != nil {
				t.shutdown(fmt.Errorf("ws ping: %w", err))
				return
			}
			timer.Reset(t.pingInterval)
		}
	}
}

func (t *wsTransport) dispatch(data []byte) {
	var probe struct {
		ID     json.RawMessage `json:"id,omitempty"`
		Method string          `json:"method,omitempty"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		// Malformed top-level JSON — drop the frame. A misbehaving
		// remote shows up as a call timeout to the caller; the log
		// is how operators distinguish "upstream silent" from
		// "upstream sending garbage".
		t.logger.Trace(
			"ws: drop unparseable frame",
			zap.Int("bytes", len(data)),
			zap.Error(err),
		)
		return
	}
	switch {
	case probe.Method == "eth_subscription":
		t.dispatchNotification(data)
	case len(probe.ID) > 0 && !bytes.Equal(probe.ID, jsonNull):
		t.dispatchResponse(data)
	default:
		t.logger.Trace(
			"ws: drop frame with no id and no recognised method",
			zap.ByteString("method", []byte(probe.Method)),
		)
	}
}

func (t *wsTransport) dispatchResponse(data []byte) {
	var resp rpcResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.logger.Trace(
			"ws: drop response (decode failed)",
			zap.Int("bytes", len(data)),
			zap.Error(err),
		)
		return
	}
	id, err := parseResponseID(resp.ID)
	if err != nil {
		t.logger.Trace(
			"ws: drop response (bad id)",
			zap.ByteString("rawID", resp.ID),
			zap.Error(err),
		)
		return
	}

	t.mu.Lock()
	ch, hasPending := t.pending[id]
	delete(t.pending, id)
	pendingSub, isSubscribe := t.pendingSubs[id]
	delete(t.pendingSubs, id)
	t.mu.Unlock()

	if !hasPending {
		// Either the caller's ctx fired and cancelPending already
		// cleaned up, or the server is replying to a request we
		// never sent. Either way nothing actionable; log so an
		// operator can correlate against client-side cancellations.
		t.logger.Trace(
			"ws: drop response (no pending caller)",
			zap.Uint64("id", id),
		)
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
		// caller doesn't get scheduled in lockstep). pendingSub.id is
		// set under t.mu so callWithSubReg can safely test for it on
		// the ctx.Done() cleanup path.
		var subID string
		if err := json.Unmarshal(resp.Result, &subID); err != nil {
			reply.err = fmt.Errorf("decode subscription id: %w", err)
		} else if subID == "" {
			reply.err = errors.New("empty subscription id")
		} else {
			t.mu.Lock()
			if t.subs != nil {
				pendingSub.id = subID
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
		t.logger.Trace(
			"ws: drop notification (decode failed)",
			zap.Int("bytes", len(data)),
			zap.Error(err),
		)
		return
	}
	t.mu.Lock()
	sub := t.subs[notif.Params.Subscription]
	t.mu.Unlock()
	if sub == nil {
		// Server may emit one more notification between our
		// eth_unsubscribe send and the server processing it; harmless,
		// but log so it's visible.
		t.logger.Trace(
			"ws: drop notification for unknown subscription",
			zap.String("subscription", notif.Params.Subscription),
		)
		return
	}
	select {
	case sub.logCh <- notif.Params.Result:
	case <-sub.closed:
	case <-t.closed:
	}
}

// shutdown is the single termination path. It fans the cause out to
// every pending caller and active subscription, then closes the conn.
//
// The cause is normalised so that errors.Is(err, ErrTransportClosed) is
// reliable for every caller observing a close — including the in-flight
// call that races the disconnect (which would otherwise see the raw
// read/ping error and skip the redial path in withRetryOnClosed).
func (t *wsTransport) shutdown(cause error) {
	t.closeOnce.Do(func() {
		switch {
		case cause == nil:
			cause = ErrTransportClosed
		case !errors.Is(cause, ErrTransportClosed):
			// fmt.Errorf with two %w verbs (Go 1.20+) wraps both errors
			// into the chain — errors.Is(err, ErrTransportClosed) is
			// reliable — while rendering on a single line. errors.Join
			// would also work but its separator is "\n", which uglifies
			// the resulting log message.
			cause = fmt.Errorf("%w: %w", ErrTransportClosed, cause)
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
// callers (only one Writer/Reader pair may be active at a time). On a
// successful write the idle ping timer is reset so we don't spend a
// ping on top of real traffic.
func (t *wsTransport) writeJSON(ctx context.Context, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if err := t.conn.Write(ctx, websocket.MessageText, data); err != nil {
		return err
	}
	select {
	case t.pingReset <- struct{}{}:
	default:
	}
	return nil
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
	ctx context.Context,
	method string,
	pendingSub *wsLogSub,
	params ...any,
) (json.RawMessage, error) {
	if params == nil {
		params = []any{}
	}
	id := t.nextID.Add(1)
	ch := make(chan rpcReply, 1)

	t.mu.Lock()
	if t.pending == nil {
		t.mu.Unlock()
		return nil, ErrTransportClosed
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
		// If the write failed because the caller's ctx was cancelled,
		// surface the ctx error verbatim — that's what the caller is
		// going to check for.
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		return nil, fmt.Errorf("write request: %w", err)
	}

	select {
	case reply := <-ch:
		// dispatchResponse already removed our entry; deregister is a no-op.
		if reply.err != nil {
			// Same race as the t.closed branch below: a cancelled-ctx
			// write can tear down the conn, the readLoop's Read fails,
			// and shutdown fans that error out to every pending caller
			// via ch — so <-ch and <-ctx.Done() are ready together and
			// the select picks at random. The caller asked about their
			// ctx; honour that rather than surfacing the resulting
			// "use of closed network connection".
			if cerr := ctx.Err(); cerr != nil {
				return nil, cerr
			}
			return nil, reply.err
		}
		return reply.result, nil
	case <-ctx.Done():
		t.cancelPending(id, pendingSub)
		return nil, ctx.Err()
	case <-t.closed:
		deregister()
		// Race: a cancelled-ctx write into coder/websocket can tear
		// down the underlying conn, which makes t.closed and
		// ctx.Done() ready simultaneously — select picks at random.
		// The caller asked about their ctx; honour that rather than
		// surfacing the transport's "use of closed network connection".
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		return nil, t.closeErr
	}
}

// cancelPending tears down a pending call after the caller's ctx fired.
// If dispatchResponse already registered a subscription, this removes it
// from t.subs and best-effort tells the server to release its side; the
// unsubscribe RPC is sent on a fresh background ctx because the caller's
// ctx is already dead.
func (t *wsTransport) cancelPending(id uint64, pendingSub *wsLogSub) {
	var leakedSubID string
	t.mu.Lock()
	if t.pending != nil {
		delete(t.pending, id)
		delete(t.pendingSubs, id)
	}
	if pendingSub != nil && pendingSub.id != "" && t.subs != nil {
		leakedSubID = pendingSub.id
		delete(t.subs, leakedSubID)
	}
	t.mu.Unlock()
	if leakedSubID == "" {
		return
	}
	// The sub was registered between dispatchResponse and the caller's
	// ctx firing; the caller will never call Unsubscribe on it, so we
	// must. Spawned because we don't want to delay the caller's return
	// past their cancelled ctx; lifetime is bounded by the unsubscribe
	// timeout and by t.closed (t.call selects on it).
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), wsUnsubscribeTimeout)
		defer cancel()
		if _, err := t.call(ctx, "eth_unsubscribe", leakedSubID); err != nil {
			t.logger.Trace(
				"ws: best-effort eth_unsubscribe failed",
				zap.String("subscription", leakedSubID),
				zap.Error(err),
			)
		}
	}()
}
