package rpcserver_test

import (
	"context"
	"net"
	"testing"

	"github.com/NethermindEth/juno/node/rpc"
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
	server, err := rpcserver.New(listener, handler, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := conc.NewWaitGroup()
	wg.Go(func() {
		err := server.Run(ctx)
		require.NoError(t, err)
	})
	t.Cleanup(wg.Wait)

	// Test that server responds appropriately for the juno_version
	// endpoint with the non-versioned route / and the versioned
	// route /0_4 for websockets and http.
	addrString := listener.Addr().String()
	for _, url := range []string{
		"ws://" + addrString,
		"ws://" + addrString + "/v0_4",
		"http://" + addrString,
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
}
