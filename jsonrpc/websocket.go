package jsonrpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NethermindEth/juno/metrics"
	"github.com/NethermindEth/juno/utils"
	"github.com/prometheus/client_golang/prometheus"
	"nhooyr.io/websocket"
)

const closeReasonMaxBytes = 125

type Websocket struct {
	rpc        *Server
	log        utils.SimpleLogger
	connParams *WebsocketConnParams
	// metrics
	requests prometheus.Counter
}

func NewWebsocket(rpc *Server, log utils.SimpleLogger) *Websocket {
	ws := &Websocket{
		rpc:        rpc,
		log:        log,
		connParams: DefaultWebsocketConnParams(),
		requests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rpc",
			Subsystem: "ws",
			Name:      "requests",
		}),
	}

	metrics.MustRegister(ws.requests)
	return ws
}

// WithConnParams sanity checks and applies the provided params.
func (ws *Websocket) WithConnParams(p *WebsocketConnParams) *Websocket {
	ws.connParams = p
	return ws
}

// ServeHTTP processes an HTTP request and upgrades it to a websocket connection.
// The connection's entire "lifetime" is spent in this function.
func (ws *Websocket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil /* TODO: options */)
	if err != nil {
		ws.log.Errorw("Failed to upgrade connection", "err", err)
		return
	}

	// TODO include connection information, such as the remote address, in the logs.

	wsc := newWebsocketConn(r.Context(), conn, ws.connParams)

	for err == nil {
		// Handle will read and write the connection once.
		err = ws.rpc.Handle(wsc.ctx, wsc)
		ws.requests.Inc()
	}

	var errClose websocket.CloseError
	if errors.As(err, &errClose) {
		ws.log.Infow("Client closed websocket connection", "status", websocket.CloseStatus(err))
		return
	}

	ws.log.Warnw("Closing websocket connection due to internal error", "err", err)
	errString := err.Error()
	if len(errString) > closeReasonMaxBytes {
		errString = errString[:closeReasonMaxBytes]
	}
	if err = wsc.conn.Close(websocket.StatusInternalError, errString); err != nil {
		// Don't log an error if the connection is already closed, which can happen
		// in benign scenarios like timeouts. Unfortunately the error is not exported
		// from the websocket package so we match the string instead.
		if !strings.Contains(err.Error(), "already wrote close") {
			ws.log.Errorw("Failed to close websocket connection", "err", err)
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
		ReadLimit:     32 * 1024 * 1024,
		WriteDuration: 5 * time.Second,
	}
}

type websocketConn struct {
	conn   *websocket.Conn
	r      io.Reader
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
	// Only one io.Reader can be open on the underlying connection
	// at a time. There is a one-to-one mapping between Readers and
	// data messages. Each time we hit an io.EOF, we create a new Reader.
	if wsc.r == nil {
		_, r, err := wsc.conn.Reader(wsc.ctx)
		if err != nil {
			return 0, err
		}
		wsc.r = r
	}
	n, err := wsc.r.Read(p)
	// We've read a full message. Create a new Reader on the next
	// call to Read.
	if err != nil && errors.Is(err, io.EOF) {
		wsc.r = nil
		err = nil
	}
	return n, err
}

// Write returns the number of bytes of p sent, not including the header.
func (wsc *websocketConn) Write(p []byte) (int, error) {
	// TODO write responses concurrently. Unlike gorilla/websocket, nhooyr.io/websocket
	// permits concurrent writes.

	writeCtx, writeCancel := context.WithTimeout(wsc.ctx, wsc.params.WriteDuration)
	defer writeCancel()
	// Use MessageText since JSON is a text format.
	err := wsc.conn.Write(writeCtx, websocket.MessageText, p)
	if err != nil {
		return 0, err
	}
	return len(p), err
}
