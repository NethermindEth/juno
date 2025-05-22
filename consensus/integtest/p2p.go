package integtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

const (
	DiscoveryServiceTag = "integration-test-discovery"
)

// discoveryNotifee gets notified when we find a new peer via mDNS discovery
type discoveryNotifee struct {
	t *testing.T
	h host.Host
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (n discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.t.Helper()
	fmt.Printf("discovered new peer %s\n", pi.ID)
	err := n.h.Connect(context.Background(), pi)
	require.NoError(n.t, err)
}

func setupP2PHost(t *testing.T) host.Host {
	addr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	require.NoError(t, err)

	p2pHost, err := libp2p.New(
		libp2p.ListenAddrs(addr),
		// libp2p.Identity(prvKey),
		// libp2p.UserAgent(makeAgentName(version)),
		// // Use address factory to add the public address to the list of
		// // addresses that the node will advertise.
		// libp2p.AddrsFactory(addressFactory),
		// If we know the public ip, enable the relay service.
		libp2p.EnableRelayService(),
		// When listening behind NAT, enable peers to try to poke thought the
		// NAT in order to reach the node.
		libp2p.EnableHolePunching(),
		// Try to open a port in the NAT router to accept incoming connections.
		libp2p.NATPortMap(),
	)
	require.NoError(t, err)

	s := mdns.NewMdnsService(p2pHost, DiscoveryServiceTag, discoveryNotifee{t: t, h: p2pHost})
	require.NoError(t, s.Start())

	return p2pHost
}
