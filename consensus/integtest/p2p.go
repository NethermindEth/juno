package integtest

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/consensus"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/stretchr/testify/require"
)

type networkNodeConfig struct {
	host          host.Host
	adjacentNodes map[int]struct{}
}

func (c networkNodeConfig) isConnected(to int) bool {
	_, ok := c.adjacentNodes[to]
	return ok
}

type networkConfig []networkNodeConfig

func (c networkConfig) setup(t *testing.T, logger utils.Logger, configFn networkConfigFn) {
	t.Helper()

	configFn(t, c)

	for _, thisNodeConfig := range c {
		peers := make([]string, 0, len(thisNodeConfig.adjacentNodes))

		for otherNode := range thisNodeConfig.adjacentNodes {
			otherNodeHost := c[otherNode].host
			peerAddr := fmt.Sprintf("%s/p2p/%s", otherNodeHost.Addrs()[0], otherNodeHost.ID())
			peers = append(peers, peerAddr)
		}

		if len(peers) > 0 {
			require.NoError(t, consensus.Connect(t.Context(), thisNodeConfig.host, strings.Join(peers, ",")))
			logger.Debugw("Connected to peers", "from", thisNodeConfig.host.ID(), "peers", peers)
		}
	}
}

func (c networkConfig) add(from, to int) {
	c[from].adjacentNodes[to] = struct{}{}
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
