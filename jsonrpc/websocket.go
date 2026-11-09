package jsonrpc

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/coder/websocket"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

const closeReasonMaxBytes = 125

var serverBusyResponse = func() []byte {
	b, err := jsonv1.Marshal(&response{
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
		var body []byte
		if body, err = ws.readMessage(wsc); err != nil {
			break
		}
		wsc.r = bytes.NewReader(body)

		ws.listener.OnNewRequest("any")
		if err = ws.handleMessage(wsc, body); err != nil {
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

// readMessage waits for the next message and reads it whole
//
// ReadAll satisfies the websocket docs' "Read to EOF otherwise
// connection will hang"
func (ws *Websocket) readMessage(wsc *websocketConn) ([]byte, error) {
	msgCtx, cancel := context.WithCancel(wsc.ctx)
	defer cancel()

	_, reader, err := wsc.conn.Reader(msgCtx)
	if err != nil {
		return nil, err
	}

	if ws.requestTimeout > 0 {
		timer := time.AfterFunc(ws.requestTimeout, cancel)
		defer timer.Stop()
	}

	return io.ReadAll(reader)
}

// busyResponse returns a server-busy response, or nil for a valid notification
// or an allowed, non-empty batch of valid notifications that must not receive a reply.
func (ws *Websocket) busyResponse(body []byte) []byte {
	body = bytes.TrimLeft(body, " \t\r\n")
	if len(body) > 0 && body[0] == '[' {
		return ws.busyBatchResponse(body)
	}

	var req Request
	if json.Unmarshal(body, &req, decodeOptions) != nil {
		return serverBusyResponse
	}
	if req.ID == nil && req.isSane() == nil {
		return nil
	}
	if req.ID == nil || !validID(req.ID) {
		return serverBusyResponse
	}

	resp := errResponse(ServerBusy, nil)
	resp.ID = req.ID
	out, err := jsonv1.Marshal(resp)
	if err != nil {
		return serverBusyResponse
	}

	return out
}

func (ws *Websocket) busyBatchResponse(body []byte) []byte {
	if ws.rpc.disableBatchRequests {
		return serverBusyResponse
	}
	decoder := jsontext.NewDecoder(bytes.NewBuffer(body), decodeOptions)
	if _, err := decoder.ReadToken(); err != nil {
		return serverBusyResponse
	}
	var req Request
	count := 0
	for decoder.PeekKind() != ']' {
		if ws.rpc.maxBatchElements > 0 && count >= ws.rpc.maxBatchElements {
			return serverBusyResponse
		}
		raw, err := decoder.ReadValue()
		if err != nil {
			return serverBusyResponse
		}
		// Unmarshal reuses its string cache instead of allocating one for each busy batch.
		req = Request{}
		if json.Unmarshal(raw, &req, decodeOptions) != nil ||
			req.ID != nil || req.isSane() != nil {
			return serverBusyResponse
		}
		count++
	}
	if count == 0 {
		return serverBusyResponse
	}
	if _, err := decoder.ReadToken(); err != nil {
		return serverBusyResponse
	}
	if _, err := decoder.ReadToken(); !errors.Is(err, io.EOF) {
		return serverBusyResponse
	}
	return nil
}

func (ws *Websocket) handleMessage(wsc *websocketConn, body []byte) error {
	if ws.gate != nil {
		if !ws.gate.TryAcquire() {
			ws.logServerBusy()
			if response := ws.busyResponse(body); response != nil {
				_, err := wsc.Write(response)
				return err
			}
			return nil
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

	// subscriptions caps the connection's subscriptions; nil leaves them unbounded
	subscriptions *semaphore.Weighted
}

var _ SubscriptionSlots = (*websocketConn)(nil)

func newWebsocketConn(
	ctx context.Context,
	conn *websocket.Conn,
	params *WebsocketConnParams,
	maxSubscriptions int64,
) *websocketConn {
	conn.SetReadLimit(params.ReadLimit)
	wsc := &websocketConn{
		conn:   conn,
		ctx:    ctx,
		params: params,
	}
	if maxSubscriptions > 0 {
		wsc.subscriptions = semaphore.NewWeighted(maxSubscriptions)
	}
	return wsc
}

// TryAcquireSubscription takes a slot if the connection is below its limit
func (wsc *websocketConn) TryAcquireSubscription() bool {
	return wsc.subscriptions == nil || wsc.subscriptions.TryAcquire(1)
}

func (wsc *websocketConn) ReleaseSubscription() {
	if wsc.subscriptions != nil {
		wsc.subscriptions.Release(1)
	}
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
