package integtest

import (
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
)

type network struct {
	proposals  map[types.Addr]listener[types.Proposal[starknet.Value]]
	prevotes   map[types.Addr]listener[types.Prevote]
	precommits map[types.Addr]listener[types.Precommit]
}

func newNetwork(allNodes nodes, buffer int) network {
	n := network{
		proposals:  make(map[types.Addr]listener[starknet.Proposal]),
		prevotes:   make(map[types.Addr]listener[types.Prevote]),
		precommits: make(map[types.Addr]listener[types.Precommit]),
	}

	for _, addr := range allNodes.addr {
		n.proposals[addr] = newListener[starknet.Proposal](buffer)
		n.prevotes[addr] = newListener[types.Prevote](buffer)
		n.precommits[addr] = newListener[types.Precommit](buffer)
	}

	return n
}

func (n network) getListeners(addr *types.Addr) Listeners {
	return Listeners{
		ProposalListener:  n.proposals[*addr],
		PrevoteListener:   n.prevotes[*addr],
		PrecommitListener: n.precommits[*addr],
	}
}

func (n network) getBroadcasters(addr *types.Addr) Broadcasters {
	return Broadcasters{
		ProposalBroadcaster:  newBroadcaster(addr, n.proposals),
		PrevoteBroadcaster:   newBroadcaster(addr, n.prevotes),
		PrecommitBroadcaster: newBroadcaster(addr, n.precommits),
	}
}
