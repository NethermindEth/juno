package integtest

import (
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
)

type network struct {
	proposals  map[felt.Felt]listener[types.Proposal[value, felt.Felt, felt.Felt]]
	prevotes   map[felt.Felt]listener[types.Prevote[felt.Felt, felt.Felt]]
	precommits map[felt.Felt]listener[types.Precommit[felt.Felt, felt.Felt]]
}

func newNetwork(allNodes nodes, buffer int) network {
	n := network{
		proposals:  make(map[felt.Felt]listener[types.Proposal[value, felt.Felt, felt.Felt]]),
		prevotes:   make(map[felt.Felt]listener[types.Prevote[felt.Felt, felt.Felt]]),
		precommits: make(map[felt.Felt]listener[types.Precommit[felt.Felt, felt.Felt]]),
	}

	for _, addr := range allNodes.addr {
		n.proposals[addr] = newListener[types.Proposal[value, felt.Felt, felt.Felt]](buffer)
		n.prevotes[addr] = newListener[types.Prevote[felt.Felt, felt.Felt]](buffer)
		n.precommits[addr] = newListener[types.Precommit[felt.Felt, felt.Felt]](buffer)
	}

	return n
}

func (n network) getListeners(addr *felt.Felt) p2p.Listeners[value, felt.Felt, felt.Felt] {
	return p2p.Listeners[value, felt.Felt, felt.Felt]{
		ProposalListener:  n.proposals[*addr],
		PrevoteListener:   n.prevotes[*addr],
		PrecommitListener: n.precommits[*addr],
	}
}

func (n network) getBroadcasters(addr *felt.Felt) p2p.Broadcasters[value, felt.Felt, felt.Felt] {
	return p2p.Broadcasters[value, felt.Felt, felt.Felt]{
		ProposalBroadcaster:  newBroadcaster(addr, n.proposals),
		PrevoteBroadcaster:   newBroadcaster(addr, n.prevotes),
		PrecommitBroadcaster: newBroadcaster(addr, n.precommits),
	}
}
