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

const (
	// wsReadLimit (16 MiB) is far above any real payload; it only stops a
	// malicious server from forcing unbounded allocations.
	wsReadLimit = 16 << 20

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

// wsTransport multiplexes unary calls over one conn, routed by request id.
type wsTransport struct {
	conn   *websocket.Conn
	nextID atomic.Uint64
	logger log.StructuredLogger

	mu      sync.Mutex
	pending map[uint64]chan rpcReply // by request id

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
	t.mu.Unlock()

	if !hasPending {
		// Caller's ctx fired before the reply landed, or an unsolicited reply.
		t.logger.Trace(
			"drop response (no pending caller)",
			zap.Uint64("id", id),
		)
		return
	}

	reply := rpcReply{}
	if resp.Error != nil {
		reply.err = rpcError{resp.Error}
	} else {
		reply.result = rawResult
	}

	// Non-blocking: ch is buffered to 1; a gone caller (ctx cancelled) leaves it unread.
	select {
	case ch <- reply:
	default:
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
		pending := t.pending
		t.pending = nil
		t.closeErr = cause
		t.mu.Unlock()
		close(t.closed)

		for _, ch := range pending {
			select {
			case ch <- rpcReply{err: cause}:
			default:
			}
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
	t.mu.Unlock()

	deregister := func() {
		t.mu.Lock()
		if t.pending != nil {
			delete(t.pending, id)
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
		deregister()
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

func isJSONNull(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || string(trimmed) == "null"
}
