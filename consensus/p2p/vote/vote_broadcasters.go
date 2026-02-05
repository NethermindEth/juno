package vote

import (
	"context"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"go.uber.org/zap"
)

type voteBroadcaster[H types.Hash, A types.Addr] struct {
	buffered.ProtoBroadcaster[*consensus.Vote]
	log         utils.Logger
	voteAdapter VoteAdapter[H, A]
}

func NewVoteBroadcaster[H types.Hash, A types.Addr](
	log utils.Logger,
	voteAdapter VoteAdapter[H, A],
	bufferSizeConfig *config.BufferSizes,
) voteBroadcaster[H, A] {
	return voteBroadcaster[H, A]{
		log:         log,
		voteAdapter: voteAdapter,
		ProtoBroadcaster: buffered.NewProtoBroadcaster(
			log,
			bufferSizeConfig.VoteProtoBroadcaster,
			bufferSizeConfig.RetryInterval,
			buffered.NewRebroadcastStrategy(bufferSizeConfig.RebroadcastInterval, func(msg *consensus.Vote) consensus.Vote_VoteType {
				return msg.VoteType
			}),
		),
	}
}

func (b *voteBroadcaster[H, A]) broadcast(ctx context.Context, message *types.Vote[H, A], voteType consensus.Vote_VoteType) {
	msg, err := b.voteAdapter.FromVote(message, voteType)
	if err != nil {
		b.log.Error("unable to convert vote", zap.Error(err))
		return
	}

	b.ProtoBroadcaster.Broadcast(ctx, &msg)
}

func (b *voteBroadcaster[H, A]) AsPrevoteBroadcaster() *prevoteBroadcaster[H, A] {
	return (*prevoteBroadcaster[H, A])(b)
}

func (b *voteBroadcaster[H, A]) AsPrecommitBroadcaster() *precommitBroadcaster[H, A] {
	return (*precommitBroadcaster[H, A])(b)
}

type prevoteBroadcaster[H types.Hash, A types.Addr] voteBroadcaster[H, A]

func (b *prevoteBroadcaster[H, A]) Broadcast(ctx context.Context, message *types.Prevote[H, A]) {
	(*voteBroadcaster[H, A])(b).broadcast(ctx, (*types.Vote[H, A])(message), consensus.Vote_Prevote)
}

type precommitBroadcaster[H types.Hash, A types.Addr] voteBroadcaster[H, A]

func (b *precommitBroadcaster[H, A]) Broadcast(ctx context.Context, message *types.Precommit[H, A]) {
	(*voteBroadcaster[H, A])(b).broadcast(ctx, (*types.Vote[H, A])(message), consensus.Vote_Precommit)
}
