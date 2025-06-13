package p2p

import (
	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/p2p/proposer"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

type proposalBroadcaster[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	buffered.ProtoBroadcaster
	log                utils.Logger
	proposalDispatcher proposer.ProposerAdapter[V, H, A]
}

func newProposalBroadcaster[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	proposalDispatcher proposer.ProposerAdapter[V, H, A],
	bufferSize int,
) proposalBroadcaster[V, H, A] {
	return proposalBroadcaster[V, H, A]{
		log:                log,
		proposalDispatcher: proposalDispatcher,
		ProtoBroadcaster:   buffered.NewProtoBroadcaster(log, bufferSize),
	}
}

func (b *proposalBroadcaster[V, H, A]) Broadcast(message types.Proposal[V, H, A]) {
	messages, err := proposer.ToProposalStream(b.proposalDispatcher, message)
	if err != nil {
		b.log.Errorw("unable to marshal proposal", "error", err)
		return
	}

	for msg, err := range messages {
		if err != nil {
			b.log.Errorw("unable to generate proposal part", "error", err)
			return
		}
		b.ProtoBroadcaster.Broadcast(msg)
	}
}
