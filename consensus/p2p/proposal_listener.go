package p2p

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/consensus/p2p/validator"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

type proposalListener[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log            utils.Logger
	proposalSub    *pubsub.Subscription
	proposalStream validator.ProposalStreamDemux[V, H, A]
	listener       listener[types.Proposal[V, H, A], V, H, A]
}

func newProposalListener[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	proposalSub *pubsub.Subscription,
	proposalStream validator.ProposalStreamDemux[V, H, A],
) *proposalListener[V, H, A] {
	return &proposalListener[V, H, A]{
		log:            log,
		proposalSub:    proposalSub,
		proposalStream: proposalStream,
		listener:       newListener[types.Proposal[V, H, A], V, H, A](),
	}
}

func (l *proposalListener[V, H, A]) loop(ctx context.Context) {
	for {
		msg, err := l.proposalSub.Next(ctx)
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			l.log.Errorw("unable to receive proposal message", "error", err)
			continue
		}

		streamMessage := consensus.StreamMessage{}
		if err := proto.Unmarshal(msg.Data, &streamMessage); err != nil {
			l.log.Errorw("unable to unmarshal proposal message", "error", err)
			continue
		}

		proposal, err := l.proposalStream.OnStreamMessage(ctx, &streamMessage)
		if err != nil {
			l.log.Errorw("unable to process proposal message", "error", err)
			continue
		}

		if proposal != nil {
			l.log.Debugw("Receive proposal", "proposal", proposal)
			l.listener.Receive(ctx, *proposal)
		}
	}
}
