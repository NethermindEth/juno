package testutils

import "math/rand"

// Because the bootstrap peers process only connect to up to 2 nodes
// See https://github.com/libp2p/go-libp2p-kad-dht/blob/v0.35.1/dht.go#L530-L541
const maxBootstrappers = 2

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

func (a AdjacentNodes) Validate() bool {
	connected := make([]map[int]struct{}, len(a))
	for from := range a {
		connected[from] = make(map[int]struct{})
	}

	for from, edges := range a {
		// Ignore all outgoing edges to nodes that have more than maxBootstrappers bootstrap peers.
		if len(edges) > maxBootstrappers {
			continue
		}

		for to := range edges {
			connected[from][to] = struct{}{}
			connected[to][from] = struct{}{}
		}
	}

	// Simple BFS to check if all nodes are connected
	visited := map[int]struct{}{0: {}}
	queue := []int{0}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		for to := range connected[from] {
			if _, ok := visited[to]; !ok {
				visited[to] = struct{}{}
				queue = append(queue, to)
			}
		}
	}

	return len(visited) == len(a)
}

// NetworkConfigFn is a function that returns an AdjacentNodes for a given number of nodes.
type NetworkConfigFn func(int) AdjacentNodes

func newNetworkConfigFn(f func(AdjacentNodes)) NetworkConfigFn {
	return func(n int) AdjacentNodes {
		for {
			adjacentNodes := NewAdjacentNodes(n)
			f(adjacentNodes)
			if adjacentNodes.Validate() {
				return adjacentNodes
			}
		}
	}
}

var LineNetworkConfig = newNetworkConfigFn(func(adjacentNodes AdjacentNodes) {
	n := len(adjacentNodes)
	for i := 0; i+1 < n; i++ {
		adjacentNodes.Connect(i, i+1)
	}
})

//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
var SmallWorldNetworkConfig = newNetworkConfigFn(func(adjacentNodes AdjacentNodes) {
	const (
		rateOfNeighborConnections = 0.2
		rateOfRandomConnections   = 0.2
	)

	n := len(adjacentNodes)
	for i := range n {
		// Connect to the next node
		adjacentNodes.ConnectRandomDirection(i, (i+1)%n)

		// 20% chance to connect to a node 2-3 hops away
		for distance := 2; distance < 4; distance++ {
			if rand.Float64() < rateOfNeighborConnections {
				adjacentNodes.ConnectRandomDirection(i, (i+distance)%n)
			}
		}

		// 20% chance to connect to a random node
		if rand.Float64() < rateOfRandomConnections {
			adjacentNodes.ConnectRandomDirection(i, rand.Intn(n))
		}
	}
})
