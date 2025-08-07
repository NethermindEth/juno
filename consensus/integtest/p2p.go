package integtest

import (
	"math/rand"
	"testing"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

type networkNodeConfig struct {
	host          host.Host
	adjacentNodes map[int]peer.AddrInfo
}

func (c networkNodeConfig) isConnected(to int) bool {
	_, ok := c.adjacentNodes[to]
	return ok
}

type networkConfig []networkNodeConfig

func (c networkConfig) add(from, to int) {
	c[from].adjacentNodes[to] = peer.AddrInfo{ID: c[to].host.ID(), Addrs: c[to].host.Addrs()}
}

//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
func (c networkConfig) addRandomDirection(from, to int) bool {
	if from == to || c[from].isConnected(to) || c[to].isConnected(from) {
		return false
	}

	if rand.Intn(2) == 0 {
		c.add(from, to)
	} else {
		c.add(to, from)
	}

	return true
}

type networkConfigFn func(t *testing.T, config networkConfig)

func lineNetworkConfig(t *testing.T, config networkConfig) {
	t.Helper()
	n := len(config)

	for i := 0; i+1 < n; i++ {
		config.add(i, i+1)
	}
}

//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
func smallWorldNetworkConfig(t *testing.T, config networkConfig) {
	t.Helper()
	n := len(config)

	rateOfNeighborConnections := 0.25
	for i := range n {
		// Connect to the next node
		require.True(t, config.addRandomDirection(i, (i+1)%n))

		// 50% chance to connect to a node 2-5 hops away
		for distance := 2; distance < 6; distance++ {
			if rand.Float64() < rateOfNeighborConnections {
				require.True(t, config.addRandomDirection(i, (i+distance)%n))
			}
		}
	}

	for i := range n {
		// Connect to 2 random nodes
		for range 2 {
			randomNode := rand.Intn(n)
			for !config.addRandomDirection(i, randomNode) {
				randomNode = rand.Intn(n)
			}
		}
	}
}
