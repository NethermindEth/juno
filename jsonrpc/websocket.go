package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/coder/websocket"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

const closeReasonMaxBytes = 125

var serverBusyResponse = func() []byte {
	b, err := json.Marshal(&response{
		Version: "2.0",
		Error:   &Error{Code: ServerBusy, Message: ErrServerBusy.Error()},
	})
	if err != nil {
		panic(err)
	}
	return b
}()

type Websocket struct {
	rpc    *Server
	logger log.StructuredLogger
	// For logging busy warnings without flooding
	sampledLogger  log.StructuredLogger
	connParams     *WebsocketConnParams
	listener       NewRequestListener
	shutdown       <-chan struct{}
	requestTimeout time.Duration
	gate           *Gate

	// connSem bounds concurrent connections
	connSem *semaphore.Weighted
	// maxSubscriptions caps every connection separately
	maxSubscriptions int64
}

func NewWebsocket(rpc *Server, shutdown <-chan struct{}, logger log.StructuredLogger) *Websocket {
	const busyLogInterval = time.Second

	ws := &Websocket{
		rpc:           rpc,
		logger:        logger,
		sampledLogger: log.Sampled(logger, busyLogInterval, 1, 0),
		connParams:    DefaultWebsocketConnParams(),
		listener:      &SelectiveListener{},
		shutdown:      shutdown,
	}

	return ws
}

// WithConnLimiter sets the semaphore that bounds concurrent websocket
// connections. nil leaves them unbounded.
func (ws *Websocket) WithConnLimiter(sem *semaphore.Weighted) *Websocket {
	ws.connSem = sem
	return ws
}

// WithMaxSubscriptions sets how many subscriptions one connection may hold at
// once; zero or less leaves them unbounded.
func (ws *Websocket) WithMaxSubscriptions(n int64) *Websocket {
	ws.maxSubscriptions = n
	return ws
}

// WithConnParams sanity checks and applies the provided params.
func (ws *Websocket) WithConnParams(p *WebsocketConnParams) *Websocket {
	ws.connParams = p
	return ws
}

func (ws *Websocket) WithRequestTimeout(d time.Duration) *Websocket {
	ws.requestTimeout = d
	return ws
}

// WithGate registers a gate
func (ws *Websocket) WithGate(g *Gate) *Websocket {
	ws.gate = g
	return ws
}

// WithListener registers a NewRequestListener
func (ws *Websocket) WithListener(listener NewRequestListener) *Websocket {
	ws.listener = listener
	return ws
}

// acquireConnSlot takes a connection slot
func (ws *Websocket) acquireConnSlot(ctx context.Context, w http.ResponseWriter) bool {
	if ws.connSem == nil {
		return true
	}

	// Create a timeout context for the acquisition
	const connTimeout = 5 * time.Second
	acquireCtx, cancel := context.WithTimeout(ctx, connTimeout)
	defer cancel()

	if err := ws.connSem.Acquire(acquireCtx, 1); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			ws.logger.Warn("Connection request timed out while waiting for slot")
			http.Error(w, "Too many connections", http.StatusServiceUnavailable)
		} else {
			ws.logger.Warn("Connection request was canceled while waiting for slot")
		}
		return false
	}

	return true
}

func (ws *Websocket) releaseConnSlot() {
	if ws.connSem != nil {
		ws.connSem.Release(1)
	}
}

// ServeHTTP processes an HTTP request and upgrades it to a websocket connection.
// The connection's entire "lifetime" is spent in this function.
func (ws *Websocket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !ws.acquireConnSlot(r.Context(), w) {
		return
	}
	defer ws.releaseConnSlot()

	conn, err := websocket.Accept(w, r, nil /* TODO: options */)
	if err != nil {
		ws.logger.Error("Failed to upgrade connection", zap.Error(err))
		return
	}

	// TODO include connection information, such as the remote address, in the logs.

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		select {
		case <-ws.shutdown:
			cancel()
		case <-ctx.Done():
			// in case websocket connection is closed and server is not in shutdown mode
			// we need to release this goroutine from waiting
		}
	}()

	wsc := newWebsocketConn(ctx, conn, ws.connParams, ws.maxSubscriptions)

	for {
		_, wsc.r, err = wsc.conn.Reader(wsc.ctx)
		if err != nil {
			break
		}
		ws.listener.OnNewRequest("any")
		if err = ws.handleMessage(wsc); err != nil {
			break
		}
		// From websocket docs: "Read to EOF otherwise connection will hang."
		if _, err = io.Copy(io.Discard, wsc.r); err != nil {
			break
		}
	}

	if status := websocket.CloseStatus(err); status != -1 {
		ws.logger.Info("Client closed websocket connection", zap.Int("status", int(status)))
		return
	}

	ws.logger.Warn("Closing websocket connection", zap.Error(err))
	errString := err.Error()
	if len(errString) > closeReasonMaxBytes {
		errString = errString[:closeReasonMaxBytes]
	}
	if err = wsc.conn.Close(websocket.StatusInternalError, errString); err != nil {
		// Don't log an error if the connection is already closed, which can happen
		// in benign scenarios like timeouts or if the underlying TCP connection was ended before the client
		// could initiate the close handshake.
		errString = err.Error()
		if !strings.Contains(errString, "already wrote close") && !strings.Contains(errString, "WebSocket closed") {
			ws.logger.Error("Failed to close websocket connection", zap.String("err", errString))
		}
	}
}

func (ws *Websocket) logServerBusy() {
	ws.sampledLogger.Warn("Rejected websocket RPC request: server is busy",
		zap.Int("running", ws.gate.Running()),
		zap.Int("queued", ws.gate.Queued()),
		zap.Uint64("rejected", ws.gate.Rejected()),
	)
}

func (ws *Websocket) handleMessage(wsc *websocketConn) error {
	if ws.gate != nil {
		if !ws.gate.TryAcquire() {
			ws.logServerBusy()
			_, err := wsc.Write(serverBusyResponse)
			return err
		}
		defer ws.gate.Release()
	}

	return ws.rpc.HandleReadWriter(wsc.ctx, ws.requestTimeout, wsc)
}

type WebsocketConnParams struct {
	// Maximum message size allowed.
	ReadLimit int64
	// Maximum time to write a message.
	WriteDuration time.Duration
}

func DefaultWebsocketConnParams() *WebsocketConnParams {
	return &WebsocketConnParams{
		ReadLimit:     32 * db.Megabyte,
		WriteDuration: 5 * time.Second,
	}
}

type websocketConn struct {
	r      io.Reader
	conn   *websocket.Conn
	ctx    context.Context
	params *WebsocketConnParams

	subscriptions    atomic.Int64
	maxSubscriptions int64
}

var _ SubscriptionSlots = (*websocketConn)(nil)

func newWebsocketConn(
	ctx context.Context,
	conn *websocket.Conn,
	params *WebsocketConnParams,
	maxSubscriptions int64,
) *websocketConn {
	conn.SetReadLimit(params.ReadLimit)
	return &websocketConn{
		conn:             conn,
		ctx:              ctx,
		params:           params,
		maxSubscriptions: maxSubscriptions,
	}
}

// TryAcquireSubscription takes a slot if the connection is below its limit
func (wsc *websocketConn) TryAcquireSubscription() bool {
	for {
		held := wsc.subscriptions.Load()
		if wsc.maxSubscriptions > 0 && held >= wsc.maxSubscriptions {
			return false
		}
		if wsc.subscriptions.CompareAndSwap(held, held+1) {
			return true
		}
	}
}

func (wsc *websocketConn) ReleaseSubscription() {
	wsc.subscriptions.Add(-1)
}

func (wsc *websocketConn) Read(p []byte) (int, error) {
	return wsc.r.Read(p)
}

// Write returns the number of bytes of p sent, not including the header.
func (wsc *websocketConn) Write(p []byte) (int, error) {
	// TODO write responses concurrently. Unlike gorilla/websocket, github.com/coder/websocket
	// permits concurrent writes.

	writeCtx, writeCancel := context.WithTimeout(wsc.ctx, wsc.params.WriteDuration)
	defer writeCancel()
	// Use MessageText since JSON is a text format.
	if err := wsc.conn.Write(writeCtx, websocket.MessageText, p); err != nil {
		return 0, err
	}
	return len(p), nil
}
