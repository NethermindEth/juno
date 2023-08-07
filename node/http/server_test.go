package junohttp_test

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/NethermindEth/juno/node/http"
	"github.com/NethermindEth/juno/rpc"
	"github.com/NethermindEth/juno/utils"
	rpcclient "github.com/ethereum/go-ethereum/rpc"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	log := utils.NewNopZapLogger()
	const version = "1.2.3-rc1"
	handler := rpc.New(nil, nil, utils.MAINNET, nil, nil, nil, version, log)

	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	server, err := junohttp.New(listener, true, handler, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := conc.NewWaitGroup()
	t.Cleanup(wg.Wait)
	wg.Go(func() {
		err := server.Run(ctx)
		require.NoError(t, err)
	})

	addrString := listener.Addr().String()
	httpAddrString := "http://" + addrString

	t.Run("rpc", func(t *testing.T) {
		// Test that server responds appropriately for the juno_version
		// endpoint with the non-versioned route / and the versioned
		// route /0_4 for websockets and http.
		for _, url := range []string{
			"ws://" + addrString,
			"ws://" + addrString + "/v0_4",
			httpAddrString,
			"http://" + addrString + "/v0_4",
		} {
			t.Run(url, func(t *testing.T) {
				// TODO using geth's rpc client for now. When we have our
				// own, use that instead. Geth pulls in a lot of dependencies
				// that we don't use.
				client, err := rpcclient.DialContext(ctx, url)
				require.NoError(t, err)
				t.Cleanup(client.Close)

				var got string
				err = client.Call(&got, "juno_version")
				require.NoError(t, err)
				assert.Equal(t, got, version)
			})
		}
	})

	t.Run("metrics", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequestWithContext(context.Background(), "GET", httpAddrString+"/metrics", http.NoBody)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, resp.Body.Close())
		})
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
