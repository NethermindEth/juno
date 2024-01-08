package node

import (
	"context"
	"net"
	"path"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestIpcService(t *testing.T) {
	t.Run("healthy service", func(t *testing.T) {
		dial := func(ctx context.Context, endpoint string) (net.Conn, error) {
			return new(net.Dialer).DialContext(ctx, "unix", endpoint)
		}
		dir := t.TempDir()
		service, err := makeRPCOverIpc(dir, map[string]*jsonrpc.Server{"/": nil, "/v0_4": nil}, utils.NewNopZapLogger(), false)
		require.NoError(t, err)
		var wg sync.WaitGroup
		defer wg.Wait()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg.Add(1)
		go func() {
			require.NoError(t, service.Run(ctx))
			wg.Done()
		}()
		// ensure that both sockets exist
		conn, err := dial(ctx, path.Join(dir, "juno.ipc"))
		require.NoError(t, err)
		require.NoError(t, conn.Close())
		conn, err = dial(ctx, path.Join(dir, "juno_v0_4.ipc"))
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})

	t.Run("handler failure", func(t *testing.T) {
		dial := func(ctx context.Context, endpoint string) (net.Conn, error) {
			return new(net.Dialer).DialContext(ctx, "unix", endpoint)
		}
		dir := t.TempDir()
		service, err := makeRPCOverIpc(dir, map[string]*jsonrpc.Server{"/v0_4": nil}, utils.NewNopZapLogger(), false)
		require.NoError(t, err)
		_, err = makeRPCOverIpc(dir, map[string]*jsonrpc.Server{"/": nil, "/v0_4": nil}, utils.NewNopZapLogger(), false)
		require.Error(t, err)

		var wg sync.WaitGroup
		defer wg.Wait()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg.Add(1)
		go func() {
			require.NoError(t, service.Run(ctx))
			wg.Done()
		}()
		// ensure that handler from second service has been cleaned up.
		_, err = dial(ctx, path.Join(dir, "juno.ipc"))
		require.Error(t, err)
		// ensure that frist service is alive.
		conn, err := dial(ctx, path.Join(dir, "juno_v0_4.ipc"))
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})
}
