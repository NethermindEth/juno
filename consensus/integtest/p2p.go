package integtest

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/require"
)

const (
	DiscoveryServiceTag = "integration-test-discovery"
)

type networkNodeConfig struct {
	index         int
	host          host.Host
	adjacentNodes map[int]struct{}
}

func (c networkNodeConfig) isConnected(to int) bool {
	_, ok := c.adjacentNodes[to]
	return ok
}

type networkConfig []networkNodeConfig

func connect(t *testing.T, mutex *sync.Mutex, thisNodeConfig, otherNodeConfig networkNodeConfig) {
	t.Helper()

	mutex.Lock()
	defer mutex.Unlock()

	peer := peer.AddrInfo{ID: otherNodeConfig.host.ID(), Addrs: otherNodeConfig.host.Addrs()}
	require.NoError(t, thisNodeConfig.host.Connect(t.Context(), peer))
}

func (c networkConfig) setup(t *testing.T) {
	t.Helper()

	mutexes := sync.Map{}
	wg := conc.NewWaitGroup()
	for thisNode, thisNodeConfig := range c {
		for otherNode := range thisNodeConfig.adjacentNodes {
			normalizedPair := fmt.Sprintf("%d-%d", min(thisNode, otherNode), max(thisNode, otherNode))
			untypedMutex, _ := mutexes.LoadOrStore(normalizedPair, &sync.Mutex{})
			mutex := untypedMutex.(*sync.Mutex)
			wg.Go(func() {
				connect(t, mutex, thisNodeConfig, c[otherNode])
			})
		}
	}
	wg.Wait()
}

func (c networkConfig) add(from, to int) {
	c[from].adjacentNodes[to] = struct{}{}
}

func initEmptyNetworkConfig(t *testing.T, n int) networkConfig {
	t.Helper()

	config := make(networkConfig, n)
	for i := range n {
		addr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
		require.NoError(t, err)

		host, err := libp2p.New(
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

		config[i] = networkNodeConfig{
			index:         i,
			host:          host,
			adjacentNodes: make(map[int]struct{}),
		}
	}

	return config
}

type networkConfigFn func(t *testing.T, n int) networkConfig

func lineNetworkConfig(t *testing.T, n int) networkConfig {
	t.Helper()

	config := initEmptyNetworkConfig(t, n)
	for i := 0; i+1 < n; i++ {
		config.add(i, i+1)
	}
	return config
}

//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
func smallWorldNetworkConfig(t *testing.T, n int) networkConfig {
	t.Helper()

	rateOfNeighborConnections := 0.25
	config := initEmptyNetworkConfig(t, n)
	for i := range n {
		// Connect to the previous node
		config.add(i, (i+n-1)%n)
		// Connect to the next node
		config.add(i, (i+1)%n)

		// 25% chance to connect to a node 2-5 hops away
		for distance := 2; distance < 6; distance++ {
			if rand.Float64() < rateOfNeighborConnections {
				config.add(i, (i+distance)%n)
			}
			if rand.Float64() < rateOfNeighborConnections {
				config.add(i, (i+n-distance)%n)
			}
		}

		// Connect to 2 random nodes
		for range 2 {
			randomNode := rand.Intn(n)
			for randomNode == i || config[i].isConnected(randomNode) {
				randomNode = rand.Intn(n)
			}
			config.add(i, randomNode)
		}
	}
	return config
}
