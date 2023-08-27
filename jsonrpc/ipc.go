package jsonrpc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

type Ipc struct {
	rpc *Server
	log utils.SimpleLogger

	endpoint   string
	connParams IpcConnParams

	listener net.Listener

	// everything below is protected
	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	closing bool
}

func NewIpc(rpc *Server, log utils.SimpleLogger, endpoint string) (*Ipc, error) {
	ipc := &Ipc{
		rpc:        rpc,
		log:        log,
		endpoint:   endpoint,
		connParams: DefaultIpcConnParams(),
		conns:      make(map[net.Conn]struct{}),
	}
	listener, err := createListener(ipc.endpoint)
	if err != nil {
		return nil, err
	}
	ipc.listener = listener
	return ipc, nil
}

func (i *Ipc) Start() {
	go i.run()
}

func (i *Ipc) run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		conn, err := i.listener.Accept()
		if netutil.IsTemporaryError(err) {
			i.log.Errorw("Failed to accept connection", "err", err)
			continue
		} else if err != nil {
			return
		}

		go i.serveConn(ctx, newIpcConn(conn, i.connParams))
	}
}

func (i *Ipc) setupConn(conn net.Conn) bool {
	// TODO include connection information, such as the remote address, in the logs.
	i.mu.Lock()
	defer i.mu.Unlock()
	i.conns[conn] = struct{}{}
	return !i.closing
}

// cleanupConn calls cleanupConnNoLock with explicit lock
func (i *Ipc) cleanupConn(conn net.Conn) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.cleanupConnNoLock(conn)
}

// cleanupConnNoLock frees resources
func (i *Ipc) cleanupConnNoLock(conn net.Conn) {
	_, ok := i.conns[conn]
	delete(i.conns, conn)
	if ok {
		conn.Close()
	}
}

func (i *Ipc) serveConn(ctx context.Context, conn net.Conn) {
	defer i.cleanupConn(conn)
	if !i.setupConn(conn) {
		return
	}
	var err error
	for err == nil {
		err = i.rpc.Handle(ctx, conn)
	}

	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		i.log.Infow("Client closed websocket connection")
		return
	}
	i.log.Warnw("Closing ipc connection due to internal error", "err", err)
}

func (i *Ipc) Stop() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closing {
		return net.ErrClosed
	}
	i.closing = true
	i.listener.Close()
	for conn := range i.conns {
		i.cleanupConnNoLock(conn)
	}
	return nil
}

type IpcConnParams struct {
	// Maximum message size allowed.
	ReadLimit int64
	// Maximum time to write a message.
	WriteDuration time.Duration
}

type ipcConn struct {
	IpcConnParams
	net.Conn
}

func DefaultIpcConnParams() IpcConnParams {
	return IpcConnParams(*DefaultWebsocketConnParams())
}

func newIpcConn(conn net.Conn, params IpcConnParams) *ipcConn {
	return &ipcConn{
		IpcConnParams: params,
		Conn:          conn,
	}
}

func (ipc *ipcConn) Read(p []byte) (int, error) {
	return ipc.Conn.Read(p)
}

// Write returns the number of bytes of p sent, not including the header.
func (ipc *ipcConn) Write(p []byte) (int, error) {
	ipc.Conn.SetWriteDeadline(time.Now().Add(ipc.WriteDuration))
	return ipc.Conn.Write(p)
}
