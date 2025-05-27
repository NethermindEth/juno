package p2p

import "github.com/NethermindEth/juno/consensus/types"

type Listener[M types.Message[V], V types.Hashable] interface {
	// Listen would return consensus messages to Tendermint which are set by the validator set.
	Listen() <-chan M
}

type Listeners[V types.Hashable] struct {
	ProposalListener  Listener[types.Proposal[V], V]
	PrevoteListener   Listener[types.Prevote, V]
	PrecommitListener Listener[types.Precommit, V]
}
