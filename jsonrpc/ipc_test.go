package jsonrpc_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/event"
	"github.com/stretchr/testify/assert"
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

func testIpc(endpoint string) (*jsonrpc.Ipc, error) {
	method := jsonrpc.Method{
		Name:   "test_echo",
		Params: []jsonrpc.Parameter{{Name: "msg"}},
		Handler: func(msg string) (string, *jsonrpc.Error) {
			return msg, nil
		},
	}
	submethod := jsonrpc.SubscribeMethod{
		Name: "test_subscribe",
		Handler: func(subServer *jsonrpc.SubscriptionServer) (event.Subscription, *jsonrpc.Error) {
			return event.NewSubscription(func(quit <-chan struct{}) error {
				for _, value := range values {
					select {
					case <-quit:
						return nil
					default:
						if err := subServer.Send(value); err != nil {
							return err
						}
					}
				}
				<-quit
				return nil
			}), nil
		},
		UnsubMethodName: "test_unsubscribe",
	}

	rpc := jsonrpc.NewServer(1, utils.NewNopZapLogger())
	rpc.WithIDGen(func() (uint64, error) {
		return id, nil
	})
	err := rpc.RegisterMethod(method)
	if err != nil {
		return nil, err
	}
	rpc.RegisterSubscribeMethod(submethod)
	return jsonrpc.NewIpc(rpc, utils.NewNopZapLogger(), endpoint)
}

func TestIpcHandler(t *testing.T) {
	const (
		msg  = `{"jsonrpc" : "2.0", "method" : "test_echo", "params" : [ "abc123" ], "id" : 1}`
		want = `{"jsonrpc":"2.0","result":"abc123","id":1}`
	)

	t.Run("single conn", func(t *testing.T) {
		ep := path.Join(t.TempDir(), "juno.ipc")
		srv, err := testIpc(ep)
		assert.NoError(t, err)
		srv.Start()
		defer func() { assert.NoError(t, srv.Stop()) }()
		conn, err := jsonrpc.IpcDial(context.Background(), ep)
		assert.NoError(t, err)
		assert.NoError(t, exchangeMsg(conn, msg, want))
	})

	t.Run("multiple conns", func(t *testing.T) {
		var (
			items = 1024
			conns = make([]net.Conn, items)
			errCh = make(chan error, items)
		)
		ep := path.Join(t.TempDir(), "juno.ipc")
		srv, err := testIpc(ep)
		assert.NoError(t, err)
		srv.Start()
		defer func() { assert.NoError(t, srv.Stop()) }()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		defer func() {
			for _, conn := range conns {
				conn.Close()
			}
		}()
		for i := 0; i < items; i++ {
			conn, err := jsonrpc.IpcDial(ctx, ep)
			assert.NoError(t, err)
			conns[i] = conn
			go func() {
				err := exchangeMsg(conn, msg, want)
				errCh <- err
			}()
		}

		for i := 0; i < items; i++ {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case err := <-errCh:
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	})

	t.Run("teardown", func(t *testing.T) {
		ep := path.Join(t.TempDir(), "juno.ipc")
		srv, err := testIpc(ep)
		assert.NoError(t, err)
		srv.Start()
		defer func() { assert.Error(t, srv.Stop()) }()
		var (
			items = 1024
			conns = make([]net.Conn, items)
			errCh = make(chan error, items)
		)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var wg sync.WaitGroup
		for i := 0; i < items; i++ {
			conn, err := jsonrpc.IpcDial(ctx, ep)
			assert.NoError(t, err)
			conns[i] = conn
			wg.Add(1)
			go func() {
				wg.Done()
				_, err := conn.Read(make([]byte, 1))
				errCh <- err
			}()
		}
		wg.Wait()
		assert.NoError(t, srv.Stop())
		for i := 0; i < items; i++ {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case err := <-errCh:
				assert.Error(t, err)
			}
		}
	})

	t.Run("disconnecting clients", func(t *testing.T) {
		ep := path.Join(t.TempDir(), "juno.ipc")
		srv, err := testIpc(ep)
		assert.NoError(t, err)
		srv.Start()
		defer func() { assert.NoError(t, srv.Stop()) }()
		var (
			items = 128
			conns = make([]net.Conn, items)
		)

		for i := 0; i < items; i++ {
			conn, err := jsonrpc.IpcDial(context.Background(), ep)
			assert.NoError(t, err)
			conns[i] = conn
		}

		for _, conn := range conns {
			assert.NoError(t, conn.Close())
		}
	})

	t.Run("subscription", func(t *testing.T) {
		ep := path.Join(t.TempDir(), "juno.ipc")
		srv, err := testIpc(ep)
		assert.NoError(t, err)
		srv.Start()
		defer func() { assert.NoError(t, srv.Stop()) }()

		conn, err := jsonrpc.IpcDial(context.Background(), ep)
		assert.NoError(t, err)

		// Initial subscription handshake.
		const req = `{"jsonrpc": "2.0", "method": "test_subscribe", "params": [], "id": 1}`
		want := fmt.Sprintf(`{"jsonrpc":"2.0","result":%d,"id":1}`, id)
		assert.NoError(t, exchangeMsg(conn, req, want))

		// All values are received.
		for _, value := range values {
			want = fmt.Sprintf(`{"jsonrpc":"2.0","method":"test_subscribe","params":{"subscription":%d,"result":%d}}`, id, value)
			buff := make([]byte, len(want))
			assert.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
			_, err := conn.Read(buff)
			assert.NoError(t, err)
			assert.Equal(t, []byte(want), buff)
		}
	})
}

func BenchmarkIpcThroughput(b *testing.B) {
	var (
		msg      = `{"jsonrpc" : "2.0", "method" : "test_echo", "params" : [ "abc123" ], "id" : 1}`
		want     = `{"jsonrpc":"2.0","result":"abc123","id":1}`
		ep       = path.Join(b.TempDir(), "juno.ipc")
		srv, err = testIpc(ep)
	)
	assert.NoError(b, err)

	srv.Start()
	defer func() { assert.NoError(b, srv.Stop()) }()

	b.ResetTimer()
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		conn, err := jsonrpc.IpcDial(context.Background(), ep)
		assert.NoError(b, err)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer conn.Close()
			assert.NoError(b, exchangeMsg(conn, msg, want))
		}()
	}
	wg.Wait()
}
