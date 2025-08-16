package testutils

import "math/rand"

// AdjacentNodes is a slice of N sets, each representing the nodes adjacent to the node at the index.
type AdjacentNodes []map[int]struct{}

// NewAdjacentNodes creates a new AdjacentNodes with N nodes with initially no connections.
func NewAdjacentNodes(n int) AdjacentNodes {
	adjacentNodes := make(AdjacentNodes, n)
	for i := range n {
		adjacentNodes[i] = make(map[int]struct{})
	}
	return adjacentNodes
}

func (a AdjacentNodes) isConnected(from, to int) bool {
	_, ok := a[from][to]
	return ok
}

func (a AdjacentNodes) Connect(from, to int) {
	a[from][to] = struct{}{}
}

//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
func (a AdjacentNodes) ConnectRandomDirection(from, to int) bool {
	if from == to || a.isConnected(from, to) || a.isConnected(to, from) {
		return false
	}

	if rand.Intn(2) == 0 {
		a.Connect(from, to)
	} else {
		a.Connect(to, from)
	}

	return true
}

// NetworkConfigFn is a function that returns an AdjacentNodes for a given number of nodes.
type NetworkConfigFn func(int) AdjacentNodes

func LineNetworkConfig(n int) AdjacentNodes {
	adjacentNodes := NewAdjacentNodes(n)

	for i := 0; i+1 < n; i++ {
		adjacentNodes.Connect(i, i+1)
	}

	return adjacentNodes
}

//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
func SmallWorldNetworkConfig(n int) AdjacentNodes {
	adjacentNodes := NewAdjacentNodes(n)

	rateOfNeighborConnections := 0.25
	for i := range n {
		// Connect to the next node
		adjacentNodes.ConnectRandomDirection(i, (i+1)%n)

		// 50% chance to connect to a node 2-5 hops away
		for distance := 2; distance < 6; distance++ {
			if rand.Float64() < rateOfNeighborConnections {
				adjacentNodes.ConnectRandomDirection(i, (i+distance)%n)
			}
		}
	}

	for i := range n {
		// Connect to 2 random nodes
		for range 2 {
			randomNode := rand.Intn(n)
			for !adjacentNodes.ConnectRandomDirection(i, randomNode) {
				randomNode = rand.Intn(n)
			}
		}
	}

	return adjacentNodes
}
