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
)

type proposalBroadcaster[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log             utils.Logger
	proposalAdapter ProposerAdapter[V, H, A]
	proposalStore   *proposal.ProposalStore[H]
	broadcaster     buffered.ProtoBroadcaster
	proposals       chan types.Proposal[V, H, A]
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
		broadcaster:     buffered.NewProtoBroadcaster(log, bufferSize, retryInterval),
		proposals:       make(chan types.Proposal[V, H, A], bufferSize),
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
				b.log.Errorw("proposal not found", "proposal", proposalHash)
				continue
			}

			dispatcher, err := newProposerDispatcher(b.proposalAdapter, &proposal, buildResult)
			if err != nil {
				b.log.Errorw("unable to build dispatcher", "error", err)
				continue
			}

			for msg, err := range dispatcher.run() {
				if err != nil {
					b.log.Errorw("unable to generate proposal part", "error", err)
					return
				}
				b.broadcaster.Broadcast(msg)
			}
		}
	}
}

func (b *proposalBroadcaster[V, H, A]) Broadcast(proposal types.Proposal[V, H, A]) {
	b.proposals <- proposal
}
