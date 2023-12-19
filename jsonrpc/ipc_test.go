package jsonrpc_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func exchangeMsg(conn net.Conn, send, want string) error {
	_, err := conn.Write([]byte(send))
	if err != nil {
		return err
	}

	buffer := make([]byte, len(want))
	_, err = conn.Read(buffer)
	if err != nil {
		return err
	}

	if !bytes.Equal(buffer, []byte(want)) {
		return fmt.Errorf("recv msg does not match. got %v, want %v", string(buffer), want)
	}
	return nil
}

// The caller is responsible for closing the server.
func testUnix(t *testing.T, method jsonrpc.Method, listener jsonrpc.EventListener, endpoint string) *jsonrpc.Ipc {
	rpc := jsonrpc.NewServer(1, utils.NewNopZapLogger()).WithListener(listener)
	require.NoError(t, rpc.RegisterMethods(method))
	ipc, err := jsonrpc.NewIpc(rpc, endpoint, utils.NewNopZapLogger())
	require.NoError(t, err)
	return ipc
}

func TestIpcHandler(t *testing.T) {
	var (
		msg    = `{"jsonrpc" : "2.0", "method" : "test_echo", "params" : [ "abc123" ], "id" : 1}`
		want   = `{"jsonrpc":"2.0","result":"abc123","id":1}`
		method = jsonrpc.Method{
			Name:   "test_echo",
			Params: []jsonrpc.Parameter{{Name: "msg"}},
			Handler: func(msg string) (string, *jsonrpc.Error) {
				return msg, nil
			},
		}
	)

	t.Run("socket cleanup", func(t *testing.T) {
		var (
			ep          = path.Join(t.TempDir(), "juno.ipc")
			ctx, cancel = context.WithCancel(context.Background())
			srvWg       sync.WaitGroup
		)
		srvWg.Add(1)

		defer cancel()
		listener := CountingEventListener{}
		srv := testUnix(t, method, &listener, ep)
		go func() {
			assert.NoError(t, srv.Run(ctx))
			srvWg.Done()
		}()

		_, err := new(net.Dialer).DialContext(context.Background(), "unix", ep)
		assert.NoError(t, err)
		cancel()
		srvWg.Wait()
		_, err = new(net.Dialer).DialContext(context.Background(), "unix", ep)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("single conn", func(t *testing.T) {
		var (
			ep          = path.Join(t.TempDir(), "juno.ipc")
			ctx, cancel = context.WithCancel(context.Background())
			srvWg       sync.WaitGroup
		)
		srvWg.Add(1)
		defer func() {
			cancel()
			srvWg.Wait()
		}()

		listener := CountingEventListener{}
		srv := testUnix(t, method, &listener, ep)
		go func() {
			assert.NoError(t, srv.Run(ctx))
			srvWg.Done()
		}()

		conn, err := new(net.Dialer).DialContext(context.Background(), "unix", ep)
		assert.NoError(t, err)
		assert.NoError(t, exchangeMsg(conn, msg, want))
	})

	t.Run("multiple conns", func(t *testing.T) {
		var (
			items       = 1024
			conns       = make([]net.Conn, items)
			errCh       = make(chan error, items)
			ep          = path.Join(t.TempDir(), "juno.ipc")
			ctx, cancel = context.WithCancel(context.Background())
			handlerWg   sync.WaitGroup
		)
		handlerWg.Add(1)
		defer func() {
			cancel()
			handlerWg.Wait()
		}()

		listener := CountingEventListener{}
		srv := testUnix(t, method, &listener, ep)
		go func() {
			assert.NoError(t, srv.Run(ctx))
			handlerWg.Done()
		}()

		for i := 0; i < items; i++ {
			conn, err := new(net.Dialer).DialContext(ctx, "unix", ep)
			assert.NoError(t, err)
			conns[i] = conn
			go func() {
				errCh <- exchangeMsg(conn, msg, want)
			}()
		}

		for i := 0; i < items; i++ {
			select {
			case <-ctx.Done():
				require.NoError(t, ctx.Err())
			case err := <-errCh:
				require.NoError(t, err)
			}
		}
		for _, conn := range conns {
			conn.Close()
		}
	})

	t.Run("teardown", func(t *testing.T) {
		var (
			ep          = path.Join(t.TempDir(), "juno.ipc")
			ctx, cancel = context.WithCancel(context.Background())
			handlerWg   sync.WaitGroup
		)
		handlerWg.Add(1)
		defer func() {
			cancel()
			handlerWg.Wait()
		}()

		listener := CountingEventListener{}
		srv := testUnix(t, method, &listener, ep)
		go func() {
			assert.NoError(t, srv.Run(ctx))
			handlerWg.Done()
		}()

		var (
			items = 1024
			conns = make([]net.Conn, items)
			errCh = make(chan error, items)
		)
		for i := 0; i < items; i++ {
			conn, err := new(net.Dialer).DialContext(ctx, "unix", ep)
			assert.NoError(t, err)
			conns[i] = conn
			go func() {
				_, err := conn.Read(make([]byte, 1))
				errCh <- err
			}()
		}
		cancel()
		for i := 0; i < items; i++ {
			err := <-errCh
			assert.Error(t, err)
		}
	})

	t.Run("disconnecting clients", func(t *testing.T) {
		var (
			ep          = path.Join(t.TempDir(), "juno.ipc")
			ctx, cancel = context.WithCancel(context.Background())
			handlerWg   sync.WaitGroup
		)
		handlerWg.Add(1)
		defer func() {
			cancel()
			handlerWg.Wait()
		}()

		listener := CountingEventListener{}
		srv := testUnix(t, method, &listener, ep)
		go func() {
			assert.NoError(t, srv.Run(ctx))
			handlerWg.Done()
		}()

		var (
			items = 128
			conns = make([]net.Conn, items)
		)
		for i := 0; i < items; i++ {
			conn, err := new(net.Dialer).DialContext(context.Background(), "unix", ep)
			assert.NoError(t, err)
			conns[i] = conn
		}

		for _, conn := range conns {
			assert.NoError(t, conn.Close())
		}
	})
	t.Run("too long path", func(t *testing.T) {
		dir := t.TempDir()
		for len(dir) <= 108 {
			dir = path.Join(dir, "foo")
		}
		_, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err := jsonrpc.NewIpc(nil, path.Join(dir, "juno.ipc"), utils.NewNopZapLogger())
		require.Error(t, err)
	})

	t.Run("already in use", func(t *testing.T) {
		dir := t.TempDir()
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		ipc, err := jsonrpc.NewIpc(nil, path.Join(dir, "juno.ipc"), utils.NewNopZapLogger())
		require.NoError(t, err)
		defer func() {
			require.NoError(t, ipc.Run(ctx))
		}()
		_, errDuplicate := jsonrpc.NewIpc(nil, path.Join(dir, "juno.ipc"), utils.NewNopZapLogger())
		require.Error(t, errDuplicate)
		cancel()
	})
}
