package integtest

import (
	"github.com/NethermindEth/juno/consensus/types"
)

type nodes struct {
	addr  []types.Addr
	index map[types.Addr]int
}

func getNodes(nodeCount int) nodes {
	nodes := nodes{
		addr:  make([]types.Addr, nodeCount),
		index: make(map[types.Addr]int),
	}
	for i := range nodeCount {
		nodes.addr[i].SetUint64(uint64(i))
		nodes.index[nodes.addr[i]] = i
	}
	return nodes
}
