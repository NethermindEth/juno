package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/gorilla/websocket"
)

type Websocket struct {
	rpc        *Server
	http       *http.Server
	log        utils.SimpleLogger
	connParams *WebsocketConnectionParams
	upgrader   *websocket.Upgrader
}

func NewWebsocket(port uint16, methods []Method, log utils.SimpleLogger) *Websocket {
	ws := &Websocket{
		rpc: NewServer(),
		log: log,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			WriteBufferPool: new(sync.Pool),
		},
		connParams: DefaultWebsocketConnectionParams(),
	}
	ws.http = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           ws,
		ReadHeaderTimeout: 1 * time.Second,
	}
	for _, method := range methods {
		err := ws.rpc.RegisterMethod(method)
		if err != nil {
			panic(err)
		}
	}
	return ws
}

func (ws *Websocket) WithConnectionParams(p *WebsocketConnectionParams) *Websocket {
	ws.connParams = p
	return ws
}

// ServeHTTP processes an HTTP request and upgrades it to a websocket connection.
// The connection's entire "lifetime" is spent in this function.
func (ws *Websocket) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.log.Errorw("Failed to upgrade connection", "err", err)
		return
	}

	wsc := newWebsocketConnection(conn, ws.rpc, ws.log, ws.connParams)
	ws.log.Infow("New websocket connection", "remote", conn.RemoteAddr())

	defer func() {
		if err := wsc.Close(); err != nil {
			ws.log.Errorw("Failed to close connection", "err", err)
		}
	}()

	go wsc.Read()
	wsc.Write() // Blocks
}

func (ws *Websocket) Run(ctx context.Context) error {
	errCh := make(chan error)

	go func() {
		<-ctx.Done()
		errCh <- ws.http.Shutdown(context.Background())
		close(errCh)
	}()

	if err := ws.http.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-errCh
}

type WebsocketConnectionParams struct {
	// How often to send pings to the client.
	PingPeriod time.Duration
	// Maximum message size allowed.
	ReadLimit int64
	// Maximum time to write a message.
	WriteDuration time.Duration
	// Maximum time to read a message.
	ReadDuration time.Duration
}

func DefaultWebsocketConnectionParams() *WebsocketConnectionParams {
	return &WebsocketConnectionParams{
		PingPeriod:    (30 * 9 * time.Second) / 10, // 0.9 * 30 seconds
		ReadLimit:     32 * 1024 * 1024,
		WriteDuration: 5 * time.Second,
		ReadDuration:  30 * time.Second,
	}
}

type websocketConnection struct {
	conn      *websocket.Conn
	writeChan chan []byte
	rpc       *Server
	log       utils.SimpleLogger
	params    *WebsocketConnectionParams
}

func newWebsocketConnection(conn *websocket.Conn, rpc *Server, log utils.SimpleLogger,
	params *WebsocketConnectionParams,
) *websocketConnection {
	conn.SetReadLimit(params.ReadLimit)
	conn.SetPongHandler(func(_ string) error {
		return conn.SetReadDeadline(time.Now().Add(params.ReadDuration))
	})
	return &websocketConnection{
		conn:      conn,
		rpc:       rpc,
		log:       log,
		writeChan: make(chan []byte),
		params:    params,
	}
}

func (wsc *websocketConnection) Read() {
	for {
		// Reset read deadline.
		if err := wsc.conn.SetReadDeadline(time.Now().Add(wsc.params.ReadDuration)); err != nil {
			wsc.log.Errorw("Failed to set read deadline", "err", err)
			return
		}

		// Read next message.
		_, r, err := wsc.conn.NextReader()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				wsc.log.Infow("Client closed the connection", "remote", wsc.conn.RemoteAddr)
			} else {
				wsc.log.Errorw("Failed to read request", "err", err, "remote", wsc.conn.RemoteAddr)
			}
			return
		}

		// Send response on the writeChan channel.
		resp, err := wsc.rpc.HandleReader(r)
		if err != nil {
			wsc.log.Errorw("Failed to create response on websocket connection", "err", err, "remote", wsc.conn.RemoteAddr)
		} else {
			wsc.writeChan <- resp
		}
	}
}

func (wsc *websocketConnection) Write() {
	sendPingTicker := time.NewTicker(wsc.params.PingPeriod)
	defer sendPingTicker.Stop()

	// https://github.com/gorilla/websocket/issues/97
	receivedPingChan := make(chan string, 1)
	wsc.conn.SetPingHandler(func(m string) error {
		select {
		case receivedPingChan <- m:
		default:
		}
		return nil
	})

	for {
		select {
		case m := <-receivedPingChan:
			if err := wsc.writeWithDeadline(websocket.PongMessage, []byte(m)); err != nil {
				wsc.log.Warnw("Failed to write pong (client may disconnect)", "err", err)
			}
		case <-sendPingTicker.C:
			if err := wsc.writeWithDeadline(websocket.PingMessage, nil); err != nil {
				wsc.log.Errorw("Failed to write ping", "err", err)
				return
			}
		case resp := <-wsc.writeChan:
			if err := wsc.writeWithDeadline(websocket.TextMessage, resp); err != nil {
				wsc.log.Errorw("Failed to write response", "err", err, "resp", resp)
				return
			}
		}
	}
}

// This is taken from cometbft.
// All writes to the websocket must (re)set the write deadline.
// If some writes don't set it while others do, they may [timeout incorrectly].
//
// [timeout incorrectly]: https://github.com/tendermint/tendermint/issues/553
func (wsc *websocketConnection) writeWithDeadline(msgType int, msg []byte) error {
	if err := wsc.conn.SetWriteDeadline(time.Now().Add(wsc.params.WriteDuration)); err != nil {
		return err
	}
	return wsc.conn.WriteMessage(msgType, msg)
}

func (wsc *websocketConnection) Close() error {
	close(wsc.writeChan)
	return wsc.conn.Close()
}
