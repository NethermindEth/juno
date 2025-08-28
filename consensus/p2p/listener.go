package p2p

import "github.com/NethermindEth/juno/consensus/types"

type Listener[M any] interface {
	// Listen would return consensus messages to Tendermint which are set by the validator set.
	Listen() <-chan M
}

type Listeners[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	ProposalListener  Listener[*types.Proposal[V, H, A]]
	PrevoteListener   Listener[*types.Prevote[H, A]]
	PrecommitListener Listener[*types.Precommit[H, A]]
}
