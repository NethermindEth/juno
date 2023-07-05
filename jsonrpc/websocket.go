package jsonrpc

import (
	"context"
	"errors"
	"net"
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
	listener   net.Listener
}

func NewWebsocket(listener net.Listener, methods []Method, log utils.SimpleLogger, validator Validator) *Websocket {
	ws := &Websocket{
		rpc:        NewServer().WithValidator(validator),
		log:        log,
		connParams: DefaultWebsocketConnParams(),
		listener:   listener,
	}
	for _, method := range methods {
		err := ws.rpc.RegisterMethod(method)
		if err != nil {
			panic(err)
		}
	}
	return ws
}

// WithConnParams sanity checks and applies the provided params.
func (ws *Websocket) WithConnParams(p *WebsocketConnParams) *Websocket {
	ws.connParams = p
	return ws
}

// Handler processes an HTTP request and upgrades it to a websocket connection.
// The connection's entire "lifetime" is spent in this function.
func (ws *Websocket) Handler(ctx context.Context) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil /* TODO: options */)
		if err != nil {
			ws.log.Errorw("Failed to upgrade connection", "err", err)
			return
		}

		// TODO include connection information, such as the remote address, in the logs.

		wsc := newWebsocketConn(conn, ws.rpc, ws.connParams)

		err = wsc.ReadWriteLoop(ctx)

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
	})
}

func (ws *Websocket) Run(ctx context.Context) error {
	errCh := make(chan error)

	srv := &http.Server{
		Handler:           ws.Handler(ctx),
		ReadHeaderTimeout: 1 * time.Second,
	}

	go func() {
		<-ctx.Done()
		errCh <- srv.Shutdown(context.Background())
		close(errCh)
	}()

	if err := srv.Serve(ws.listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-errCh
}

type WebsocketConnParams struct {
	// Maximum message size allowed.
	ReadLimit int64
	// Maximum time to write a message.
	WriteDuration time.Duration
	// Maximum time to read a message.
	ReadDuration time.Duration
}

func DefaultWebsocketConnParams() *WebsocketConnParams {
	return &WebsocketConnParams{
		ReadLimit:     32 * 1024 * 1024,
		WriteDuration: 5 * time.Second,
		ReadDuration:  30 * time.Second,
	}
}

type websocketConn struct {
	conn   *websocket.Conn
	rpc    *Server
	params *WebsocketConnParams
}

func newWebsocketConn(conn *websocket.Conn, rpc *Server, params *WebsocketConnParams) *websocketConn {
	conn.SetReadLimit(params.ReadLimit)
	return &websocketConn{
		conn:   conn,
		rpc:    rpc,
		params: params,
	}
}

func (wsc *websocketConn) ReadWriteLoop(ctx context.Context) error {
	for {
		// Read next message from the client.
		_, r, err := wsc.Read(ctx)
		if err != nil {
			return err
		}

		// TODO write responses concurrently. Unlike gorilla/websocket, nhooyr.io/websocket
		// permits concurrent writes.

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

func (wsc *websocketConn) Read(ctx context.Context) (websocket.MessageType, []byte, error) {
	readCtx, readCancel := context.WithTimeout(ctx, wsc.params.ReadDuration)
	defer readCancel()
	return wsc.conn.Read(readCtx)
}

func (wsc *websocketConn) Write(ctx context.Context, msg []byte) error {
	writeCtx, writeCancel := context.WithTimeout(ctx, wsc.params.WriteDuration)
	defer writeCancel()
	// Use MessageText since JSON is a text format.
	return wsc.conn.Write(writeCtx, websocket.MessageText, msg)
}
