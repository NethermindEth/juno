package p2p

import "github.com/NethermindEth/juno/consensus/types"

type Broadcaster[M types.Message[V, H, A], V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// Broadcast will broadcast the message to the whole validator set. The function should not be blocking.
	Broadcast(M)
}

type Broadcasters[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	ProposalBroadcaster  Broadcaster[types.Proposal[V, H, A], V, H, A]
	PrevoteBroadcaster   Broadcaster[types.Prevote[H, A], V, H, A]
	PrecommitBroadcaster Broadcaster[types.Precommit[H, A], V, H, A]
}
