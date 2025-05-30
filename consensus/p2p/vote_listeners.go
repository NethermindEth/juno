package p2p

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/consensus/p2p/vote"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

type voteListeners[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log               utils.Logger
	voteSub           *pubsub.Subscription
	voteAdapter       vote.VoteAdapter[H, A]
	prevoteListener   listener[types.Prevote[H, A], V, H, A]
	precommitListener listener[types.Precommit[H, A], V, H, A]
}

func newVoteListeners[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	voteSub *pubsub.Subscription,
	voteAdapter vote.VoteAdapter[H, A],
) *voteListeners[V, H, A] {
	return &voteListeners[V, H, A]{
		log:               log,
		voteSub:           voteSub,
		voteAdapter:       voteAdapter,
		prevoteListener:   newListener[types.Prevote[H, A], V, H, A](),
		precommitListener: newListener[types.Precommit[H, A], V, H, A](),
	}
}

func (l *voteListeners[V, H, A]) loop(ctx context.Context) {
	for {
		msg, err := l.voteSub.Next(ctx)
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			l.log.Errorw("unable to receive vote message", "error", err)
			continue
		}

		p2pVote := consensus.Vote{}
		if err := proto.Unmarshal(msg.Data, &p2pVote); err != nil {
			l.log.Errorw("unable to unmarshal vote message", "error", err)
			continue
		}

		vote, err := l.voteAdapter.ToVote(&p2pVote)
		if err != nil {
			l.log.Errorw("unable to convert vote message to vote", "error", err)
			continue
		}

		switch p2pVote.VoteType {
		case consensus.Vote_Prevote:
			l.log.Debugw("Receive prevote", "vote", vote)
			l.prevoteListener.Receive(ctx, types.Prevote[H, A](vote))
		case consensus.Vote_Precommit:
			l.log.Debugw("Receive precommit", "vote", vote)
			l.precommitListener.Receive(ctx, types.Precommit[H, A](vote))
		}
	}
}
