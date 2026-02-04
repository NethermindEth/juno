package proposer

import (
	"context"
	"time"

	"github.com/NethermindEth/juno/consensus/p2p/buffered"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/sourcegraph/conc"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"go.uber.org/zap"
)

type proposalBroadcaster[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log             utils.Logger
	proposalAdapter ProposerAdapter[V, H, A]
	proposalStore   *proposal.ProposalStore[H]
	broadcaster     buffered.ProtoBroadcaster[*consensus.StreamMessage]
	proposals       chan *types.Proposal[V, H, A]
}

func NewProposalBroadcaster[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	proposalAdapter ProposerAdapter[V, H, A],
	proposalStore *proposal.ProposalStore[H],
	bufferSize int,
	retryInterval time.Duration,
) proposalBroadcaster[V, H, A] {
	return proposalBroadcaster[V, H, A]{
		log:             log,
		proposalAdapter: proposalAdapter,
		proposalStore:   proposalStore,
		broadcaster:     buffered.NewProtoBroadcaster[*consensus.StreamMessage](log, bufferSize, retryInterval, nil),
		proposals:       make(chan *types.Proposal[V, H, A], bufferSize),
	}
}

func (b *proposalBroadcaster[V, H, A]) Loop(ctx context.Context, topic *pubsub.Topic) {
	wg := conc.NewWaitGroup()
	wg.Go(func() {
		b.broadcaster.Loop(ctx, topic)
	})

	wg.Go(func() {
		b.processLoop(ctx)
	})

	wg.Wait()
}

func (b *proposalBroadcaster[V, H, A]) processLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case proposal := <-b.proposals:
			proposalHash := (*proposal.Value).Hash()
			buildResult := b.proposalStore.Get(proposalHash)
			if buildResult == nil {
				// todo(rdr): need to update the constrains of `H` to be able to log it
				b.log.Error("proposal not found", zap.Any("proposal", proposalHash))
				continue
			}

			dispatcher, err := newProposerDispatcher(b.proposalAdapter, proposal, buildResult)
			if err != nil {
				b.log.Error("unable to build dispatcher", zap.Error(err))
				continue
			}

			for msg, err := range dispatcher.run() {
				if err != nil {
					b.log.Error("unable to generate proposal part", zap.Error(err))
					return
				}
				b.broadcaster.Broadcast(ctx, msg)
			}
		}
	}
}

func (b *proposalBroadcaster[V, H, A]) Broadcast(ctx context.Context, proposal *types.Proposal[V, H, A]) {
	select {
	case <-ctx.Done():
		return
	case b.proposals <- proposal:
	}
}
