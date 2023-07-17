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

func NewWebsocket(listener net.Listener, rpc *Server, log utils.SimpleLogger) *Websocket {
	ws := &Websocket{
		rpc:        rpc,
		log:        log,
		connParams: DefaultWebsocketConnParams(),
		listener:   listener,
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

		wsc := newWebsocketConn(ctx, conn, ws.rpc, ws.connParams)

		err = wsc.ReadWriteLoop()

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
	ctx    context.Context
	rpc    *Server
	params *WebsocketConnParams
}

func newWebsocketConn(ctx context.Context, conn *websocket.Conn, rpc *Server, params *WebsocketConnParams) *websocketConn {
	conn.SetReadLimit(params.ReadLimit)
	return &websocketConn{
		conn:   conn,
		ctx:    ctx,
		rpc:    rpc,
		params: params,
	}
}

func (wsc *websocketConn) ReadWriteLoop() error {
	var err error
	for err == nil {
		// Handle will read and write the connection once.
		err = wsc.rpc.Handle(wsc)
	}
	return err
}

func (wsc *websocketConn) Read(p []byte) (int, error) {
	readCtx, readCancel := context.WithTimeout(wsc.ctx, wsc.params.ReadDuration)
	defer readCancel()
	var err error
	_, r, err := wsc.conn.Read(readCtx)
	return copy(p, r), err // TODO how can we remove this copy?
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
