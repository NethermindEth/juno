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

type voteListener[M any] chan M

func newListener[M any](bufferSize int) voteListener[M] {
	return voteListener[M](make(chan M, bufferSize))
}

func (l voteListener[M]) Listen() <-chan M {
	return l
}

func (l voteListener[M]) Receive(ctx context.Context, message M) {
	select {
	case <-ctx.Done():
		return
	case l <- message:
	}
}

type voteListeners[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	buffered.TopicSubscription
	log               utils.Logger
	PrevoteListener   voteListener[*types.Prevote[H, A]]
	PrecommitListener voteListener[*types.Precommit[H, A]]
}

func NewVoteListeners[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	voteAdapter VoteAdapter[H, A],
	bufferSizeConfig *config.BufferSizes,
) voteListeners[V, H, A] {
	prevoteListener := newListener[*types.Prevote[H, A]](bufferSizeConfig.PrevoteOutput)
	precommitListener := newListener[*types.Precommit[H, A]](bufferSizeConfig.PrecommitOutput)

	onMessage := func(ctx context.Context, msg *pubsub.Message) {
		p2pVote := consensus.Vote{}
		if err := proto.Unmarshal(msg.Data, &p2pVote); err != nil {
			log.Error("unable to unmarshal vote message", utils.SugaredFields("error", err)...)
			return
		}

		vote, err := voteAdapter.ToVote(&p2pVote)
		if err != nil {
			log.Error("unable to convert vote message to vote", utils.SugaredFields("error", err)...)
			return
		}

		switch p2pVote.VoteType {
		case consensus.Vote_Prevote:
			prevoteListener.Receive(ctx, (*types.Prevote[H, A])(&vote))
		case consensus.Vote_Precommit:
			precommitListener.Receive(ctx, (*types.Precommit[H, A])(&vote))
		}
	}

	return voteListeners[V, H, A]{
		TopicSubscription: buffered.NewTopicSubscription(log, bufferSizeConfig.VoteSubscription, onMessage),
		log:               log,
		PrevoteListener:   prevoteListener,
		PrecommitListener: precommitListener,
	}
}
