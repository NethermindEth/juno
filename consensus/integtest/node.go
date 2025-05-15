package integtest

import (
	"github.com/NethermindEth/juno/core/felt"
)

type nodes struct {
	addr  []felt.Felt
	index map[felt.Felt]int
}

func getNodes(nodeCount int) nodes {
	nodes := nodes{
		addr:  make([]felt.Felt, nodeCount),
		index: make(map[felt.Felt]int),
	}
	for i := range nodeCount {
		(&nodes.addr[i]).SetUint64(uint64(i))
		nodes.index[nodes.addr[i]] = i
	}
	return nodes
}
