package jsonrpc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/NethermindEth/juno/utils"
	"nhooyr.io/websocket"
)

const closeReasonMaxBytes = 125

type Websocket struct {
	rpc        *Server
	log        utils.SimpleLogger
	connParams *WebsocketConnParams
	listener   NewRequestListener
}

func NewWebsocket(rpc *Server, log utils.SimpleLogger) *Websocket {
	ws := &Websocket{
		rpc:        rpc,
		log:        log,
		connParams: DefaultWebsocketConnParams(),
		listener:   &SelectiveListener{},
	}

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
	conn, err := websocket.Accept(w, r, nil /* TODO: options */)
	if err != nil {
		ws.log.Errorw("Failed to upgrade connection", "err", err)
		return
	}

	// TODO include connection information, such as the remote address, in the logs.

	wsc := newWebsocketConn(r.Context(), conn, ws.connParams)

	ctx := context.WithValue(wsc.ctx, ConnKey{}, wsc)
	for {
		var msg []byte
		_, msg, err = wsc.conn.Read(ctx)
		if err != nil {
			break
		}
		var resp []byte
		resp, err = ws.rpc.Handle(ctx, msg)
		if err != nil {
			break
		}
		ws.listener.OnNewRequest("any")
		if _, err = wsc.Write(resp); err != nil {
			break
		}
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

// Write returns the number of bytes of p sent, not including the header.
func (wsc *websocketConn) Write(p []byte) (int, error) {
	// TODO write responses concurrently. Unlike gorilla/websocket, nhooyr.io/websocket
	// permits concurrent writes.

	writeCtx, writeCancel := context.WithTimeout(wsc.ctx, wsc.params.WriteDuration)
	defer writeCancel()
	// Use MessageText since JSON is a text format.
	if err := wsc.conn.Write(writeCtx, websocket.MessageText, p); err != nil {
		return 0, err
	}
	return len(p), nil
}
