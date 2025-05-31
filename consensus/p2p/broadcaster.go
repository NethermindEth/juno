package p2p

import "github.com/NethermindEth/juno/consensus/types"

type Broadcaster[M types.Message[V], V types.Hashable] interface {
	// Broadcast will broadcast the message to the whole validator set. The function should not be blocking.
	Broadcast(M)
}

type Broadcasters[V types.Hashable] struct {
	ProposalBroadcaster  Broadcaster[types.Proposal[V], V]
	PrevoteBroadcaster   Broadcaster[types.Prevote, V]
	PrecommitBroadcaster Broadcaster[types.Precommit, V]
}
