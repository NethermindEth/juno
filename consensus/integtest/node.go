package integtest

import (
	"github.com/NethermindEth/juno/consensus/starknet"
)

type nodes struct {
	addr  []starknet.Address
	index map[starknet.Address]int
}

func getNodes(nodeCount int) nodes {
	nodes := nodes{
		addr:  make([]starknet.Address, nodeCount),
		index: make(map[starknet.Address]int),
	}
	for i := range nodeCount {
		nodes.addr[i].AsFelt().SetUint64(uint64(i))
		nodes.index[nodes.addr[i]] = i
	}
	return nodes
}
