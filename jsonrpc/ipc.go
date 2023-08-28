package jsonrpc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/NethermindEth/juno/metrics"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/prometheus/client_golang/prometheus"
)

type Ipc struct {
	rpc *Server
	log utils.SimpleLogger

	connParams IpcConnParams
	listener   net.Listener

	// metrics
	requests prometheus.Counter

	// everything below is protected
	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	closing bool
}

func NewIpc(rpc *Server, log utils.SimpleLogger, endpoint string) (*Ipc, error) {
	ipc := &Ipc{
		rpc:        rpc,
		log:        log,
		connParams: DefaultIpcConnParams(),
		conns:      make(map[net.Conn]struct{}),
		requests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rpc",
			Subsystem: "ipc",
			Name:      "requests",
		}),
	}
	listener, err := createListener(endpoint)
	if err != nil {
		return nil, err
	}
	ipc.listener = listener

	metrics.MustRegister(ipc.requests)

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
		i.requests.Inc()
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
	// Maximum time to write a message.
	WriteDuration time.Duration
}

type ipcConn struct {
	IpcConnParams
	net.Conn
}

func DefaultIpcConnParams() IpcConnParams {
	return IpcConnParams{
		WriteDuration: 5 * time.Second,
	}
}

func newIpcConn(conn net.Conn, params IpcConnParams) *ipcConn {
	return &ipcConn{
		IpcConnParams: params,
		Conn:          conn,
	}
}

func (ipc *ipcConn) Write(p []byte) (int, error) {
	ipc.Conn.SetWriteDeadline(time.Now().Add(ipc.WriteDuration))
	return ipc.Conn.Write(p)
}
