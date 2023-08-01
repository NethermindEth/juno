package jsonrpc

import (
	"context"
	"errors"
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

	wsc := newWebsocketConn(conn, ws.rpc, ws.connParams, ws.requests)

	err = wsc.ReadWriteLoop(r.Context())

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
	conn     *websocket.Conn
	rpc      *Server
	params   *WebsocketConnParams
	requests prometheus.Counter
}

func newWebsocketConn(conn *websocket.Conn, rpc *Server, params *WebsocketConnParams, requests prometheus.Counter) *websocketConn {
	conn.SetReadLimit(params.ReadLimit)
	return &websocketConn{
		conn:     conn,
		rpc:      rpc,
		params:   params,
		requests: requests,
	}
}

func (wsc *websocketConn) ReadWriteLoop(ctx context.Context) error {
	for {
		// Read next message from the client.
		_, r, err := wsc.conn.Read(ctx)
		if err != nil {
			return err
		}

		// TODO write responses concurrently. Unlike gorilla/websocket, nhooyr.io/websocket
		// permits concurrent writes.

		wsc.requests.Inc()
		// Decode the message, call the handler, encode the response.
		resp, err := wsc.rpc.Handle(r)
		if err != nil {
			// RPC handling issues should not affect the connection.
			// Ignore the request and let the client close the connection.
			// TODO: is there a better way to do this?
			continue
		}

		// Send the message to the client.
		if err := wsc.Write(ctx, resp); err != nil {
			return err
		}
	}
}

func (wsc *websocketConn) Write(ctx context.Context, msg []byte) error {
	writeCtx, writeCancel := context.WithTimeout(ctx, wsc.params.WriteDuration)
	defer writeCancel()
	// Use MessageText since JSON is a text format.
	return wsc.conn.Write(writeCtx, websocket.MessageText, msg)
}
