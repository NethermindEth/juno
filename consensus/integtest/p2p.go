package integtest

import (
	"fmt"
	"math/rand"
	"sync"
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

type networkNodeConfig struct {
	t             *testing.T
	host          host.Host
	adjacentNodes map[string]struct{}
	mu            *sync.Mutex
}

// HandlePeerFound connects to peers discovered via mDNS. Once they're connected,
// the PubSub system will automatically start interacting with them if they also
// support PubSub.
func (c networkNodeConfig) HandlePeerFound(pi peer.AddrInfo) {
	if !c.isConnected(pi.ID.String()) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t.Helper()
	err := c.host.Connect(c.t.Context(), pi)
	fmt.Println("connect error", c.host.ID().ShortString(), pi.ID.ShortString(), err)
	require.NoError(c.t, err)
}

func (c networkNodeConfig) isConnected(to string) bool {
	_, ok := c.adjacentNodes[to]
	return ok
}

func (c networkNodeConfig) setupDiscovery() {
	s := mdns.NewMdnsService(c.host, DiscoveryServiceTag, c)
	c.t.Cleanup(func() {
		require.NoError(c.t, s.Close())
	})
	require.NoError(c.t, s.Start())
}

type networkConfig []networkNodeConfig

func (c networkConfig) add(from, to int) {
	fmt.Println("adding edge", from, to)
	c[from].adjacentNodes[c[to].host.ID().String()] = struct{}{}
}

func initEmptyNetworkConfig(t *testing.T, n int) networkConfig {
	t.Helper()

	mu := &sync.Mutex{}

	config := make(networkConfig, n)
	for i := range n {
		addr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
		require.NoError(t, err)

		config[i].t = t
		config[i].adjacentNodes = make(map[string]struct{})
		config[i].mu = mu

		config[i].host, err = libp2p.New(
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
	}

	return config
}

type networkConfigFn func(t *testing.T, n int) networkConfig

func lineNetworkConfig(t *testing.T, n int) networkConfig {
	config := initEmptyNetworkConfig(t, n)
	for i := 0; i+1 < n; i++ {
		config.add(i, i+1)
	}
	return config
}

func smallWorldNetworkConfig(t *testing.T, n int) networkConfig {
	config := initEmptyNetworkConfig(t, n)
	for i := range n {
		// Connect to the previous node
		config.add(i, (i+n-1)%n)
		// Connect to the next node
		config.add(i, (i+1)%n)

		// 25% chance to connect to a node 2-5 hops away
		for distance := 2; distance < 6; distance++ {
			if rand.Float64() < 0.25 {
				config.add(i, (i+distance)%n)
			}
			if rand.Float64() < 0.25 {
				config.add(i, (i+n-distance)%n)
			}
		}

		// Connect to 2 random nodes
		for range 2 {
			randomNode := rand.Intn(n)
			for randomNode == i || config[i].isConnected(config[randomNode].host.ID().String()) {
				randomNode = rand.Intn(n)
			}
			config.add(i, randomNode)
		}
	}
	return config
}
