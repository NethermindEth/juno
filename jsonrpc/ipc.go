package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/sourcegraph/conc"
)

const (
	// On Linux, path is 108 bytes in size.
	// http://man7.org/linux/man-pages/man7/unix.7.html
	maxUnixPathBytes = 108
	// fileModePerms represents the permission mode for directories, which is set to 751.
	// This mode is stricter than the common default of 755 for directories.
	fileModePerms = 0o751
	// socketPerms represents the permission mode for socket files, set to 600. It allows
	// read and write access for the owner only, ensuring that socket is accessible only to the intended user.
	socketPerms = 0o600
)

// createListener creates a Unix domain socket at the given path and sets proper permissions.
func createListener(endpoint string) (net.Listener, error) {
	// path + terminator
	if len(endpoint)+1 > maxUnixPathBytes {
		return nil, errors.New("path too long")
	}
	// Try connecting first; if it works, then the socket is still live,
	// so let's abort the creation of a new one.
	if c, err := net.Dial("unix", endpoint); err == nil {
		c.Close()
		return nil, fmt.Errorf("%v: address already in use", endpoint)
	}

	if err := os.MkdirAll(filepath.Dir(endpoint), fileModePerms); err != nil {
		return nil, err
	}
	// Remove any existing file at the specified path.
	if err := os.Remove(endpoint); !os.IsNotExist(err) {
		return nil, err
	}
	l, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, err
	}
	// Set permissions for the socket file to read and write for the owner only (0o600)
	err = os.Chmod(endpoint, socketPerms)
	return l, err
}

type Ipc struct {
	rpc    *Server
	events NewRequestListener
	log    utils.SimpleLogger

	connWg conc.WaitGroup // connWg is a WaitGroup for tracking active connections.

	connParams IpcConnParams
	listener   net.Listener

	// everything below is protected
	mu    sync.Mutex
	conns map[net.Conn]struct{} // conns is a map that holds active connections.
}

// NewIpc creates a new IPC handler instance with the provided RPC server, endpoint and logger.
func NewIpc(rpc *Server, endpoint string, log utils.SimpleLogger) (*Ipc, error) {
	listener, err := createListener(endpoint)
	if err != nil {
		return nil, err
	}
	return &Ipc{
		rpc:        rpc,
		log:        log,
		connParams: DefaultIpcConnParams(),
		conns:      make(map[net.Conn]struct{}),
		events:     &SelectiveListener{},
		listener:   listener,
	}, nil
}

// WithConnParams sanity checks and applies the provided params.
func (i *Ipc) WithConnParams(p *IpcConnParams) *Ipc {
	i.connParams = *p
	return i
}

// WithListener registers a NewRequestListener
func (i *Ipc) WithListener(listener NewRequestListener) *Ipc {
	i.events = listener
	return i
}

// Run launches the IPC handler and handles any potential errors.
// It is the caller's responsibility to cancel the provided context when they wish to
// gracefully shut down the IPC handler.
func (i *Ipc) Run(ctx context.Context) error {
	var wg conc.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg.Go(func() {
		defer cancel()
		for {
			conn, err := i.listener.Accept()
			if netutil.IsTemporaryError(err) {
				i.log.Warnw("Failed to accept connection", "err", err)
				continue
			} else if err != nil {
				if !isSocketError(err) {
					i.log.Warnw("Accept connection", "err", err)
				}
				return
			}
			i.connWg.Go(func() {
				i.serveConn(ctx, newIpcConn(conn, i.connParams))
			})
		}
	})

	<-ctx.Done()
	return i.stop()
}

// cleanupConnNoLock frees resources
func (i *Ipc) cleanupConnNoLock(conn net.Conn) {
	_, ok := i.conns[conn]
	delete(i.conns, conn)
	if !ok {
		return
	}
	if err := conn.Close(); err != nil {
		i.log.Warnw("Error occurred while closing connection", "err", err)
	}
}

// serveConn handles incoming connection.
func (i *Ipc) serveConn(ctx context.Context, conn net.Conn) {
	defer func() {
		i.mu.Lock()
		defer i.mu.Unlock()
		i.cleanupConnNoLock(conn)
	}()
	i.mu.Lock()
	i.conns[conn] = struct{}{}
	i.mu.Unlock()
	if ctx.Err() != nil {
		return
	}

	var err error
	for err == nil {
		i.events.OnNewRequest("any")
		err = i.rpc.HandleReadWriter(ctx, conn)
	}

	if isSocketError(err) || isPipeError(err) {
		return
	}

	i.log.Warnw("Closing ipc connection due to internal error", "err", err)
}

// stop gracefully shuts down the IPC handler.
func (i *Ipc) stop() error {
	i.mu.Lock()
	defer func() {
		i.mu.Unlock()
		i.connWg.Wait()
	}()
	err := i.listener.Close()
	for conn := range i.conns {
		i.cleanupConnNoLock(conn)
	}
	return err
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
	if err := ipc.Conn.SetWriteDeadline(time.Now().Add(ipc.WriteDuration)); err != nil {
		return 0, err
	}
	return ipc.Conn.Write(p)
}

func isSocketError(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF)
}

func isPipeError(err error) bool {
	// broken pipe || conn reset
	return errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)
}
