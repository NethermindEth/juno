package jsonrpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/coder/websocket"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

const (
	closeReasonMaxBytes = 125
	maxConns            = 2048 // TODO: an arbitrary default number, should be revisited after monitoring
)

type Websocket struct {
	rpc        *Server
	log        utils.StructuredLogger
	connParams *WebsocketConnParams
	listener   NewRequestListener
	shutdown   <-chan struct{}

	// Add connection tracking
	connSem *semaphore.Weighted
}

func NewWebsocket(rpc *Server, shutdown <-chan struct{}, log utils.StructuredLogger) *Websocket {
	ws := &Websocket{
		rpc:        rpc,
		log:        log,
		connParams: DefaultWebsocketConnParams(),
		listener:   &SelectiveListener{},
		shutdown:   shutdown,
		connSem:    semaphore.NewWeighted(maxConns),
	}

	return ws
}

// WithMaxConnections sets the maximum number of concurrent websocket connections
func (ws *Websocket) WithMaxConnections(maxConns int64) *Websocket {
	ws.connSem = semaphore.NewWeighted(maxConns)
	return ws
}

// WithConnParams sanity checks and applies the provided params.
func (ws *Websocket) WithConnParams(p *WebsocketConnParams) *Websocket {
	ws.connParams = p
	return ws
}

// WithListener registers a NewRequestListener
func (ws *Websocket) WithListener(listener NewRequestListener) *Websocket {
	ws.listener = listener
	return ws
}

// ServeHTTP processes an HTTP request and upgrades it to a websocket connection.
// The connection's entire "lifetime" is spent in this function.
func (ws *Websocket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create a timeout context for the acquisition
	const connTimeout = 5 * time.Second
	acquireCtx, cancel := context.WithTimeout(r.Context(), connTimeout)
	defer cancel()

	// Check connection limit
	if err := ws.connSem.Acquire(acquireCtx, 1); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			ws.log.Warn("Connection request timed out while waiting for slot")
			http.Error(w, "Too many connections", http.StatusServiceUnavailable)
		} else {
			ws.log.Warn("Connection request was canceled while waiting for slot")
		}
		return
	}
	defer ws.connSem.Release(1)

	conn, err := websocket.Accept(w, r, nil /* TODO: options */)
	if err != nil {
		ws.log.Error("Failed to upgrade connection", zap.Error(err))
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

	wsc := newWebsocketConn(ctx, conn, ws.connParams)

	for {
		_, wsc.r, err = wsc.conn.Reader(wsc.ctx)
		if err != nil {
			break
		}
		ws.listener.OnNewRequest("any")
		if err = ws.rpc.HandleReadWriter(wsc.ctx, wsc); err != nil {
			break
		}
		// From websocket docs: "Read to EOF otherwise connection will hang."
		if _, err = io.Copy(io.Discard, wsc.r); err != nil {
			break
		}
	}

	if status := websocket.CloseStatus(err); status != -1 {
		ws.log.Info("Client closed websocket connection", zap.Int("status", int(status)))
		return
	}

	ws.log.Warn("Closing websocket connection", zap.Error(err))
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
			ws.log.Error("Failed to close websocket connection", zap.String("err", errString))
		}
	}
}

type WebsocketConnParams struct {
	// Maximum message size allowed.
	ReadLimit int64
	// Maximum time to write a message.
	WriteDuration time.Duration
}

func DefaultWebsocketConnParams() *WebsocketConnParams {
	return &WebsocketConnParams{
		ReadLimit:     32 * utils.Megabyte,
		WriteDuration: 5 * time.Second,
	}
}

type websocketConn struct {
	r      io.Reader
	conn   *websocket.Conn
	ctx    context.Context
	params *WebsocketConnParams
}

func newWebsocketConn(ctx context.Context, conn *websocket.Conn, params *WebsocketConnParams) *websocketConn {
	conn.SetReadLimit(params.ReadLimit)
	return &websocketConn{
		conn:   conn,
		ctx:    ctx,
		params: params,
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
