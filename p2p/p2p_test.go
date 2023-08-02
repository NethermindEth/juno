package p2p_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
	testCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	peerA, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"peerA",
		"",
		"",
		utils.INTEGRATION,
		utils.NewNopZapLogger(),
	)
	require.NoError(t, err)

	events, err := peerA.SubscribePeerConnectednessChanged(testCtx)
	require.NoError(t, err)

	bootAddrs, err := peerA.ListenAddrs()
	require.NoError(t, err)

	var bootAddrsString []string
	for _, bootAddr := range bootAddrs {
		bootAddrsString = append(bootAddrsString, bootAddr.String())
	}

	peerB, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30302",
		"peerB",
		strings.Join(bootAddrsString, ","),
		"",
		utils.INTEGRATION,
		utils.NewNopZapLogger(),
	)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		require.NoError(t, peerA.Run(testCtx))
	}()
	go func() {
		defer wg.Done()
		require.NoError(t, peerB.Run(testCtx))
	}()

	select {
	case evt := <-events:
		require.Equal(t, network.Connected, evt.Connectedness)
	case <-time.After(time.Second):
		require.True(t, false, "no events were emitted")
	}

	t.Run("new stream", func(t *testing.T) {
		stream, err := peerA.NewStream(testCtx, identify.ID)
		require.NoError(t, err)
		require.NoError(t, stream.Close())

		stream, err = peerB.NewStream(testCtx, identify.ID)
		require.NoError(t, err)
		require.NoError(t, stream.Close())
	})

	cancel()
	wg.Wait()
}

func TestInvalidKey(t *testing.T) {
	_, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"peerA",
		"",
		"something",
		utils.INTEGRATION,
		utils.NewNopZapLogger(),
	)

	require.Error(t, err)
}

func TestValidKey(t *testing.T) {
	_, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"peerA",
		"",
		"08011240333b4a433f16d7ca225c0e99d0d8c437b835cb74a98d9279c561977690c80f681b25ccf3fa45e2f2de260149c112fa516b69057dd3b0151a879416c0cb12d9b3",
		utils.INTEGRATION,
		utils.NewNopZapLogger(),
	)

	require.NoError(t, err)
}
