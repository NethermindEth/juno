package p2p

import (
	"context"

	"github.com/NethermindEth/juno/consensus/types"
)

type Broadcaster[M any] interface {
	// Broadcast will broadcast the message to the whole validator set. The function should not be blocking.
	Broadcast(context.Context, M)
}

type Broadcasters[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	ProposalBroadcaster  Broadcaster[*types.Proposal[V, H, A]]
	PrevoteBroadcaster   Broadcaster[*types.Prevote[H, A]]
	PrecommitBroadcaster Broadcaster[*types.Precommit[H, A]]
}
