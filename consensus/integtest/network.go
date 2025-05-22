package integtest

import (
	"github.com/NethermindEth/juno/consensus/starknet"
)

type network struct {
	proposals  map[starknet.Address]listener[starknet.Proposal]
	prevotes   map[starknet.Address]listener[starknet.Prevote]
	precommits map[starknet.Address]listener[starknet.Precommit]
}

func newNetwork(allNodes nodes, buffer int) network {
	n := network{
		proposals:  make(map[starknet.Address]listener[starknet.Proposal]),
		prevotes:   make(map[starknet.Address]listener[starknet.Prevote]),
		precommits: make(map[starknet.Address]listener[starknet.Precommit]),
	}

	for _, addr := range allNodes.addr {
		n.proposals[addr] = newListener[starknet.Proposal](buffer)
		n.prevotes[addr] = newListener[starknet.Prevote](buffer)
		n.precommits[addr] = newListener[starknet.Precommit](buffer)
	}

	return n
}

func (n network) getListeners(addr *starknet.Address) Listeners {
	return Listeners{
		ProposalListener:  n.proposals[*addr],
		PrevoteListener:   n.prevotes[*addr],
		PrecommitListener: n.precommits[*addr],
	}
}

func (n network) getBroadcasters(addr *starknet.Address) Broadcasters {
	return Broadcasters{
		ProposalBroadcaster:  newBroadcaster(addr, n.proposals),
		PrevoteBroadcaster:   newBroadcaster(addr, n.prevotes),
		PrecommitBroadcaster: newBroadcaster(addr, n.precommits),
	}
}
