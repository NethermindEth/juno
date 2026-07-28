package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/coder/websocket"
	"go.uber.org/zap"
)

var ErrTransportClosed = errors.New("transport closed")

// ErrSubscriptionQueueOverflow fails a subscription whose sink is not drained
// fast enough, matching go-ethereum's behaviour.
var ErrSubscriptionQueueOverflow = errors.New("subscription queue overflow (slow subscriber)")

const (
	// wsReadLimit (16 MiB) is far above any real payload; it only stops a
	// malicious server from forcing unbounded allocations.
	wsReadLimit = 16 << 20

	wsLogSubBuffer = 64

	// maxOrphanedSubs bounds the orphaned-subscribe tracking against a server
	// that never answers; past the cap a late reply leaks its server-side sub,
	// which such a server was never going to honour anyway.
	maxOrphanedSubs = 1024

	wsUnsubscribeTimeout = 2 * time.Second

	wsPingInterval = 30 * time.Second
	wsPingTimeout  = 10 * time.Second
	wsDialTimeout  = time.Minute

	// wsWriteTimeout bounds a frame write independently of any caller's ctx:
	// coder/websocket closes the whole conn if a write ctx cancels mid-frame,
	// so one caller's cancel must not flap the shared conn.
	wsWriteTimeout = 10 * time.Second
)

type rpcReply struct {
	result json.RawMessage
	err    error
}

// rpcError adapts a server jsonrpc.Error into a Go error at the client boundary.
type rpcError struct{ err *jsonrpc.Error }

func (e rpcError) Error() string {
	if e.err.Data != nil {
		return fmt.Sprintf("jsonrpc %d: %s: %v", e.err.Code, e.err.Message, e.err.Data)
	}
	return fmt.Sprintf("jsonrpc %d: %s", e.err.Code, e.err.Message)
}

// wsTransport multiplexes unary calls and eth_subscribe notifications over
// one conn, routed by request id / subscription id.
type wsTransport struct {
	conn   *websocket.Conn
	nextID atomic.Uint64
	logger log.StructuredLogger

	mu      sync.Mutex
	pending map[uint64]chan rpcReply // by request id
	// pendingSubs: subscribe calls awaiting the server-assigned sub id, keyed by request id.
	pendingSubs map[uint64]*wsLogSub
	// orphanedSubs: request ids of cancelled subscribes whose reply never arrived. A late
	// successful reply still triggers a best-effort eth_unsubscribe, without retaining the
	// full wsLogSub.
	orphanedSubs map[uint64]struct{}
	subs         map[string]*wsLogSub // active subscriptions, by sub id

	pingInterval time.Duration
	pingTimeout  time.Duration

	closed chan struct{}
	// cancelLoops ends readLoop and pingLoop; called from shutdown.
	cancelLoops context.CancelFunc
	closeErr    error
	closeOnce   sync.Once
}

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
	dialTimeout := opts.dialTimeout
	if dialTimeout <= 0 {
		dialTimeout = wsDialTimeout
	}
	dialCtx, cancelDial := context.WithTimeout(ctx, dialTimeout)
	defer cancelDial()
	// nil DialOptions is deliberate: juno's endpoints authenticate via
	// key-in-URL (no custom headers), and compression stays off by default.
	conn, resp, err := websocket.Dial(dialCtx, rawURL, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("dialing ws: %w", err)
	}
	conn.SetReadLimit(wsReadLimit)
	t := &wsTransport{
		conn:         conn,
		logger:       opts.logger,
		pending:      make(map[uint64]chan rpcReply),
		pendingSubs:  make(map[uint64]*wsLogSub),
		orphanedSubs: make(map[uint64]struct{}),
		subs:         make(map[string]*wsLogSub),
		pingInterval: opts.pingInterval,
		pingTimeout:  opts.pingTimeout,
		closed:       make(chan struct{}),
	}
	// The loops outlive every caller by design — the Client retires them via
	// Close or redial (shutdown cancels) — so Background is their true parent.
	loopCtx, cancel := context.WithCancel(context.Background())
	t.cancelLoops = cancel
	go t.readLoop(loopCtx) //nolint:gosec // G118: long-lived loop, not request-scoped
	go t.pingLoop(loopCtx) //nolint:gosec // G118: long-lived loop, not request-scoped
	return t, nil
}

// readLoop drops malformed frames rather than tearing the transport down —
// a misbehaving remote manifests as a call timeout.
func (t *wsTransport) readLoop(ctx context.Context) {
	for {
		_, data, err := t.conn.Read(ctx)
		if err != nil {
			t.shutdown(err)
			return
		}
		t.dispatch(data)
	}
}

// pingLoop pings unconditionally: a ping is a round-trip probe, so it also
// catches half-open conns that successful writes alone would mask. A ping
// failure shuts the transport down via the same path as a read error.
func (t *wsTransport) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(t.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, t.pingTimeout)
			err := t.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				t.shutdown(fmt.Errorf("pinging ws: %w", err))
				return
			}
		}
	}
}

func (t *wsTransport) dispatch(data []byte) {
	var probe struct {
		ID     json.RawMessage `json:"id,omitempty"`
		Method string          `json:"method,omitempty"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		t.logger.Trace(
			"drop unparseable frame",
			zap.Int("bytes", len(data)),
			zap.Error(err),
		)
		return
	}
	switch {
	case probe.Method == "eth_subscription":
		t.dispatchNotification(data)
	case len(probe.ID) > 0 && !isJSONNull(probe.ID):
		t.dispatchResponse(data, probe.ID)
	default:
		t.logger.Trace(
			"drop frame with no id and no recognised method",
			zap.ByteString("method", []byte(probe.Method)),
		)
	}
}

func (t *wsTransport) dispatchResponse(data []byte, rawID json.RawMessage) {
	id, err := strconv.ParseUint(string(rawID), 10, 64)
	if err != nil {
		t.logger.Trace(
			"drop response (bad id)",
			zap.ByteString("rawID", rawID),
			zap.Error(err),
		)
		return
	}
	var rawResult json.RawMessage
	resp := jsonrpc.Response{Result: &rawResult}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.logger.Trace(
			"drop response (decode failed)",
			zap.Int("bytes", len(data)),
			zap.Error(err),
		)
		return
	}

	t.mu.Lock()
	ch, hasPending := t.pending[id]
	delete(t.pending, id)
	pendingSub, isSubscribe := t.pendingSubs[id]
	delete(t.pendingSubs, id)
	_, isOrphaned := t.orphanedSubs[id]
	delete(t.orphanedSubs, id)
	t.mu.Unlock()

	if !hasPending {
		// Caller's ctx fired before the reply landed (or an unsolicited reply). A successful
		// subscribe orphaned a server-side sub the caller will never own — release it.
		if (isSubscribe || isOrphaned) && resp.Error == nil {
			if subID, derr := decodeSubID(rawResult); derr == nil {
				t.unsubscribeInBackground(subID)
			}
		}
		t.logger.Trace(
			"drop response (no pending caller)",
			zap.Uint64("id", id),
		)
		return
	}

	reply := rpcReply{}
	switch {
	case resp.Error != nil:
		reply.err = rpcError{resp.Error}
	case isSubscribe:
		// Register the sub before waking the caller, else a notification could race in ahead of it.
		if subID, derr := decodeSubID(rawResult); derr != nil {
			reply.err = derr
		} else if t.registerSub(pendingSub, subID) {
			reply.result = rawResult
		} else {
			// Transport closed or caller cancelled; registerSub released any orphaned server-side sub.
			reply.err = ErrTransportClosed
		}
	default:
		reply.result = rawResult
	}

	// Non-blocking: ch is buffered to 1; a gone caller (ctx cancelled) leaves it unread.
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
			"drop notification (decode failed)",
			zap.Int("bytes", len(data)),
			zap.Error(err),
		)
		return
	}
	t.mu.Lock()
	sub := t.subs[notif.Params.Subscription]
	t.mu.Unlock()
	if sub == nil {
		// Late notification between our eth_unsubscribe and the server processing it. Harmless.
		t.logger.Trace(
			"drop notification for unknown subscription",
			zap.String("subscription", notif.Params.Subscription),
		)
		return
	}
	select {
	case sub.logCh <- notif.Params.Result:
	default:
		// Full buffer: fail the slow subscription rather than block the shared
		// readLoop, which would stall every unary call on the connection.
		sub.fail(ErrSubscriptionQueueOverflow)
		t.removeSub(sub)
	}
}

func (t *wsTransport) removeSub(s *wsLogSub) {
	t.mu.Lock()
	id := s.id
	s.id = ""
	if t.subs != nil && id != "" {
		delete(t.subs, id)
	}
	t.mu.Unlock()
	if id != "" {
		t.unsubscribeInBackground(id)
	}
}

// shutdown is the single termination path. The cause is normalised so
// errors.Is(err, ErrTransportClosed) holds for every observer, including the
// in-flight call that races the disconnect and must redial.
func (t *wsTransport) shutdown(cause error) {
	t.closeOnce.Do(func() {
		switch {
		case cause == nil:
			cause = ErrTransportClosed
		case !errors.Is(cause, ErrTransportClosed):
			// Wrap so errors.Is holds; single-line (errors.Join splits across log lines).
			cause = fmt.Errorf("%w: %w", ErrTransportClosed, cause)
		}
		t.mu.Lock()
		pending, pendingSubs, subs := t.pending, t.pendingSubs, t.subs
		t.pending = nil
		t.pendingSubs = nil
		t.orphanedSubs = nil
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
		if t.cancelLoops != nil {
			t.cancelLoops()
		}
		// CloseNow: don't block on a handshake the remote may have abandoned.
		_ = t.conn.CloseNow()
	})
}

func (t *wsTransport) close() { t.shutdown(ErrTransportClosed) }

// writeJSON bounds writes by wsWriteTimeout, not the caller's ctx (see the
// const); caller cancellation applies only while awaiting the reply.
func (t *wsTransport) writeJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), wsWriteTimeout)
	defer cancel()
	if err := t.conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.shutdown(fmt.Errorf("writing frame: %w", err))
		return fmt.Errorf("%w: writing frame: %w", ErrTransportClosed, err)
	}
	return nil
}

func (t *wsTransport) call(
	ctx context.Context,
	method string,
	params ...any,
) (json.RawMessage, error) {
	return t.callWithSubReg(ctx, method, nil, params...)
}

// callWithSubReg registers a non-nil pendingSub atomically with the reply
// delivery, so no notification can arrive before the sub is routable.
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

	if err := t.writeJSON(jsonrpc.Request{
		Version: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}); err != nil {
		deregister()
		return nil, err
	}

	select {
	case reply := <-ch:
		// dispatchResponse already removed our entry; deregister is a no-op.
		if reply.err != nil {
			// A concurrent shutdown fans an error into ch as the caller cancels; prefer the ctx error.
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
		// Same ctx/close race as the reply branch above; prefer the
		// caller's ctx error.
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		return nil, t.closeErr
	}
}

// registerSub returns false (releasing the server-side sub) if the transport
// closed or the caller cancelled. Atomic against cancelPending under t.mu:
// whoever locks first wins.
func (t *wsTransport) registerSub(pendingSub *wsLogSub, subID string) bool {
	t.mu.Lock()
	switch {
	case t.subs == nil:
		t.mu.Unlock()
		return false
	case pendingSub.cancelled:
		t.mu.Unlock()
		t.unsubscribeInBackground(subID)
		return false
	default:
		pendingSub.id = subID
		t.subs[subID] = pendingSub
		t.mu.Unlock()
		return true
	}
}

// cancelPending tears down a pending call whose caller's ctx fired, releasing
// any server-side sub it would otherwise leak.
func (t *wsTransport) cancelPending(id uint64, pendingSub *wsLogSub) {
	var leakedSubID string
	t.mu.Lock()
	if t.pending != nil {
		delete(t.pending, id)
	}
	if pendingSub != nil {
		pendingSub.cancelled = true
		if _, awaitingReply := t.pendingSubs[id]; awaitingReply {
			delete(t.pendingSubs, id)
			if len(t.orphanedSubs) < maxOrphanedSubs {
				t.orphanedSubs[id] = struct{}{}
			}
		}
		if pendingSub.id != "" && t.subs != nil {
			leakedSubID = pendingSub.id
			delete(t.subs, leakedSubID)
		}
	}
	t.mu.Unlock()
	if leakedSubID == "" {
		return
	}
	t.unsubscribeInBackground(leakedSubID)
}

// unsubscribeInBackground never delays the caller; bounded by wsUnsubscribeTimeout.
func (t *wsTransport) unsubscribeInBackground(subID string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), wsUnsubscribeTimeout)
		defer cancel()
		if _, err := t.call(ctx, "eth_unsubscribe", subID); err != nil {
			t.logger.Trace(
				"best-effort eth_unsubscribe failed",
				zap.String("subscription", subID),
				zap.Error(err),
			)
		}
	}()
}

func decodeSubID(raw json.RawMessage) (string, error) {
	var subID string
	if err := json.Unmarshal(raw, &subID); err != nil {
		return "", fmt.Errorf("decoding subscription id: %w", err)
	}
	if subID == "" {
		return "", errors.New("empty subscription id")
	}
	return subID, nil
}

func isJSONNull(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || string(trimmed) == "null"
}
