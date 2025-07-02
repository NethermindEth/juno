package vote

import (
	"context"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

type VoteListener[M types.Message[V, H, A], V types.Hashable[H], H types.Hash, A types.Addr] chan M

func newListener[M types.Message[V, H, A], V types.Hashable[H], H types.Hash, A types.Addr](bufferSize int) VoteListener[M, V, H, A] {
	return VoteListener[M, V, H, A](make(chan M, bufferSize))
}

func (l VoteListener[M, V, H, A]) Listen() <-chan M {
	return l
}

func (l VoteListener[M, V, H, A]) Receive(ctx context.Context, message M) {
	select {
	case <-ctx.Done():
		return
	case l <- message:
	}
}

type VoteListeners[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	buffered.TopicSubscription
	log                   utils.Logger
	PrevoteListener       VoteListener[types.Prevote[H, A], V, H, A]
	PrecommitListener     VoteListener[types.Precommit[H, A], V, H, A]
	SyncPrevoteListener   VoteListener[types.Prevote[H, A], V, H, A]
	SyncPrecommitListener VoteListener[types.Precommit[H, A], V, H, A]
}

func NewVoteListeners[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	voteAdapter VoteAdapter[H, A],
	bufferSizeConfig *config.BufferSizes,
) VoteListeners[V, H, A] {
	prevoteListener := newListener[types.Prevote[H, A], V, H, A](bufferSizeConfig.PrevoteOutput)
	precommitListener := newListener[types.Precommit[H, A], V, H, A](bufferSizeConfig.PrecommitOutput)

	syncPrevoteListener := newListener[types.Prevote[H, A], V, H, A](bufferSizeConfig.PrevoteOutput)
	syncPrecommitListener := newListener[types.Precommit[H, A], V, H, A](bufferSizeConfig.PrecommitOutput)

	onMessage := func(ctx context.Context, msg *pubsub.Message) {
		p2pVote := consensus.Vote{}
		if err := proto.Unmarshal(msg.Data, &p2pVote); err != nil {
			log.Errorw("unable to unmarshal vote message", "error", err)
			return
		}

		vote, err := voteAdapter.ToVote(&p2pVote)
		if err != nil {
			log.Errorw("unable to convert vote message to vote", "error", err)
			return
		}

		switch p2pVote.VoteType {
		case consensus.Vote_Prevote:
			prevoteListener.Receive(ctx, types.Prevote[H, A](vote))
		case consensus.Vote_Precommit:
			precommitListener.Receive(ctx, types.Precommit[H, A](vote))
		}

		switch p2pVote.VoteType {
		case consensus.Vote_Prevote:
			syncPrevoteListener.Receive(ctx, types.Prevote[H, A](vote))
		case consensus.Vote_Precommit:
			syncPrecommitListener.Receive(ctx, types.Precommit[H, A](vote))
		}
	}

	return VoteListeners[V, H, A]{
		TopicSubscription:     buffered.NewTopicSubscription(log, bufferSizeConfig.VoteSubscription, onMessage),
		log:                   log,
		PrevoteListener:       prevoteListener,
		PrecommitListener:     precommitListener,
		SyncPrevoteListener:   syncPrevoteListener,
		SyncPrecommitListener: syncPrecommitListener,
	}
}
