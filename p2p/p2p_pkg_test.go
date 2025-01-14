package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
	"github.com/stretchr/testify/require"
)

func TestListenForL1Events(t *testing.T) {
	eventChan := make(chan BootnodeRegistryEvent, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		close(eventChan)
	}()

	net, err := mocknet.FullMeshLinked(2)
	require.NoError(t, err)
	peerHosts := net.Hosts()
	require.Len(t, peerHosts, 2)
	testDB := pebble.NewMemTest(t)

	peerA, err := NewWithHost(peerHosts[0], "", false, nil, &utils.Integration, utils.NewNopZapLogger(), testDB, eventChan)
	require.NoError(t, err)

	go peerA.listenForL1Events(ctx)

	peerB, err := NewWithHost(peerHosts[1], "", false, nil, &utils.Integration, utils.NewNopZapLogger(), testDB, nil)
	require.NoError(t, err)

	addr, err := peerB.ListenAddrs()
	require.NoError(t, err)
	addrStr := addr[0].String()

	eventChan <- BootnodeRegistryEvent{Add, addrStr}
	time.Sleep(100 * time.Millisecond)
	expectedPeers := map[peer.ID]struct{}{
		peerHosts[0].ID(): {},
		peerHosts[1].ID(): {},
	}

	for _, peer := range peerA.host.Peerstore().Peers() {
		require.Contains(t, expectedPeers, peer)
	}

	// TODO: test remove event, find a way to remove peer from peerstore
}
