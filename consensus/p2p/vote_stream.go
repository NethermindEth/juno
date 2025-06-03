package p2p

import (
	"context"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/p2p/vote"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

type voteListener[M types.Message[V, H, A], V types.Hashable[H], H types.Hash, A types.Addr] chan M

func newListener[M types.Message[V, H, A], V types.Hashable[H], H types.Hash, A types.Addr](bufferSize int) voteListener[M, V, H, A] {
	return voteListener[M, V, H, A](make(chan M, bufferSize))
}

func (l voteListener[M, V, H, A]) Listen() <-chan M {
	return l
}

func (l voteListener[M, V, H, A]) Receive(ctx context.Context, message M) {
	select {
	case <-ctx.Done():
		return
	case l <- message:
	}
}

type voteStream[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log               utils.Logger
	voteAdapter       vote.VoteAdapter[H, A]
	prevoteListener   voteListener[types.Prevote[H, A], V, H, A]
	precommitListener voteListener[types.Precommit[H, A], V, H, A]
}

func newVoteStream[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	voteAdapter vote.VoteAdapter[H, A],
	bufferSizeConfig *buffered.BufferSizeConfig,
) *voteStream[V, H, A] {
	return &voteStream[V, H, A]{
		log:               log,
		voteAdapter:       voteAdapter,
		prevoteListener:   newListener[types.Prevote[H, A], V, H, A](bufferSizeConfig.PrevoteOutput),
		precommitListener: newListener[types.Precommit[H, A], V, H, A](bufferSizeConfig.PrecommitOutput),
	}
}

func (l *voteStream[V, H, A]) OnMessage(ctx context.Context, msg *pubsub.Message) {
	p2pVote := consensus.Vote{}
	if err := proto.Unmarshal(msg.Data, &p2pVote); err != nil {
		l.log.Errorw("unable to unmarshal vote message", "error", err)
		return
	}

	vote, err := l.voteAdapter.ToVote(&p2pVote)
	if err != nil {
		l.log.Errorw("unable to convert vote message to vote", "error", err)
		return
	}

	switch p2pVote.VoteType {
	case consensus.Vote_Prevote:
		l.prevoteListener.Receive(ctx, types.Prevote[H, A](vote))
	case consensus.Vote_Precommit:
		l.precommitListener.Receive(ctx, types.Precommit[H, A](vote))
	}
}
