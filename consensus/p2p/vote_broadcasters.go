package p2p

import (
	"github.com/NethermindEth/juno/consensus/p2p/vote"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

type voteBroadcaster[H types.Hash, A types.Addr] struct {
	protoBroadcaster
	log         utils.Logger
	voteAdapter vote.VoteAdapter[H, A]
}

func newVoteBroadcaster[H types.Hash, A types.Addr](
	log utils.Logger,
	voteAdapter vote.VoteAdapter[H, A],
	topic *pubsub.Topic,
) voteBroadcaster[H, A] {
	return voteBroadcaster[H, A]{
		log:              log,
		voteAdapter:      voteAdapter,
		protoBroadcaster: newProtoBroadcaster(log, topic),
	}
}

func (b *voteBroadcaster[H, A]) broadcast(message *types.Vote[H, A], voteType consensus.Vote_VoteType) {
	msg, err := b.voteAdapter.FromVote(message, voteType)
	if err != nil {
		b.log.Errorw("unable to convert vote", "error", err)
		return
	}

	b.protoBroadcaster.broadcast(&msg)
}

type prevoteBroadcaster[H types.Hash, A types.Addr] voteBroadcaster[H, A]

func (b *prevoteBroadcaster[H, A]) Broadcast(message types.Prevote[H, A]) {
	(*voteBroadcaster[H, A])(b).broadcast((*types.Vote[H, A])(&message), consensus.Vote_Prevote)
}

type precommitBroadcaster[H types.Hash, A types.Addr] voteBroadcaster[H, A]

func (b *precommitBroadcaster[H, A]) Broadcast(message types.Precommit[H, A]) {
	(*voteBroadcaster[H, A])(b).broadcast((*types.Vote[H, A])(&message), consensus.Vote_Precommit)
}
