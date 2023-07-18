package p2p_test

import (
	"context"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db/pebble"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/p2p"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
	db, err := pebble.NewMem()
	if err != nil {
		panic(err)
	}
	bc := blockchain.New(db, utils.INTEGRATION, utils.NewNopZapLogger())

	testCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	peerA, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"peerA",
		"",
		"",
		bc,
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
		bc,
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

	cancel()
	wg.Wait()
}

func TestInvalidKey(t *testing.T) {
	_, err := p2p.New(
		"/ip4/127.0.0.1/tcp/30301",
		"peerA",
		"",
		"something",
		nil,
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
		nil,
		utils.INTEGRATION,
		utils.NewNopZapLogger(),
	)

	require.NoError(t, err)
}
