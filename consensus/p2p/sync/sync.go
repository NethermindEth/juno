package sync

import (
	"context"
	"sync/atomic"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/p2p/vote"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	p2pSync "github.com/NethermindEth/juno/p2p"
	junoSync "github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/log"
)

const fPlus1 = 3 // Todo

type Service[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	driverSyncListener    chan V                     // Notify the Driver that we have fallen behind, and should commit the provided block
	proposalStore         *proposal.ProposalStore[H] // So the Driver can see the block we need to commit
	syncPrevoteListener   vote.VoteListener[types.Prevote[H, A], V, H, A]
	syncPrecommitListener vote.VoteListener[types.Precommit[H, A], V, H, A]
	p2pSync               p2pSync.Service
	messages              map[types.Height][]A // height-> []peers
	curHeight             atomic.Uint64
	driverCommitNotifier  chan struct{} // Prevents this service falling behind the State StateMachine // Todo: Driver needs to ping this
	log                   utils.Logger
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	driverSyncListener chan V,
	syncPrevoteListener vote.VoteListener[types.Prevote[H, A], V, H, A],
	syncPrecommitListener vote.VoteListener[types.Precommit[H, A], V, H, A],
	proposalStore *proposal.ProposalStore[H],
	p2pSync p2pSync.Service,
	curHeight uint64,
	commitNotifier chan struct{},
	log utils.Logger,
) *Service[V, H, A] {
	srv := Service[V, H, A]{
		driverSyncListener:    driverSyncListener,
		syncPrevoteListener:   syncPrevoteListener,
		syncPrecommitListener: syncPrecommitListener,
		proposalStore:         proposalStore,
		p2pSync:               p2pSync,
		messages:              make(map[types.Height][]A),
		driverCommitNotifier:  commitNotifier,
		log:                   log,
	}
	srv.curHeight.Store(curHeight)
	return &srv
}

func (s *Service[V, H, A]) Run(ctx context.Context) {
	networkHeightCh := make(chan uint64)

	// 1) Collect votes & push futureHeight updates
	go func() {
		defer close(networkHeightCh)
		for {
			select {
			// case msg := <-s.syncPrevoteListener.Listen(): // Todo: actually implement this
			// 	if msg.Height > types.Height(s.curHeight.Load()) {
			// 		s.messages[msg.Height] = append(s.messages[msg.Height], msg.Sender)
			// 	}
			case msg := <-s.syncPrevoteListener.Listen():
				if msg.Height > types.Height(s.curHeight.Load()) {
					s.messages[msg.Height] = append(s.messages[msg.Height], msg.Sender)
				}
			case msg := <-s.syncPrecommitListener.Listen():
				if msg.Height > types.Height(s.curHeight.Load()) {
					s.messages[msg.Height] = append(s.messages[msg.Height], msg.Sender)
				}
			case <-s.driverCommitNotifier:
				s.deleteOldMessages(types.Height(s.curHeight.Load()))
				s.curHeight.Add(1)
			case <-ctx.Done():
				return
			}

			// once we see > f+1 at some height, push it
			for height, senders := range s.messages {
				if len(senders) > fPlus1 {
					select {
					case networkHeightCh <- uint64(height):
					case <-ctx.Done():
						return
					}
					s.deleteOldMessages(height)
					break
				}
			}
		}
	}()

	// 2) Sync blocks up to each announced height
	for future := range networkHeightCh {
		for s.curHeight.Load() < future {
			blocksCh, err := s.p2pSync.GetBlock(ctx, s.curHeight.Load()+1)
			if err != nil {
				log.Error("failed to sync block", "err", err)
				continue
			}
			for b := range blocksCh {
				// Peers don't send this, so we must recalculate it.
				concatCount := core.ConcatCounts(
					b.Block.TransactionCount,
					b.Block.EventCount,
					b.StateUpdate.StateDiff.Length(),
					b.Block.L1DAMode,
				)
				block := builder.BuildResult{
					Pending: &junoSync.Pending{
						Block:       b.Block,
						StateUpdate: b.StateUpdate,
						NewClasses:  b.NewClasses,
					},
					SimulateResult: &blockchain.SimulateResult{
						ConcatCount:      concatCount,
						BlockCommitments: b.Commitments,
					},
					// Todo(!) : unless our peers give us this value, we can only get it via re-execution, which
					// we shouldn't do. The spec should be updated. For now, we can set it to zero so that
					// we can correctly process empty blocks. For non-empty blocks we should ignore this
					// specific check (in Commit()) until the spec is updated (ie or peers communicate it)
					L2GasConsumed: uint64(0),
				}

				// Approach 1: store the proposal/block. Notify the Driver that the proposalStore has been updated, and to commit the block
				hash := H(*block.Pending.Block.Hash)
				valueHash := *new(V) // Todo: pass in block hash
				s.proposalStore.Store(hash, &block)

				s.driverSyncListener <- valueHash

				// Approach 2: somehow reconstruct all the precommits and pass that into the Driver...
				// 1. we don't know which round the block is associated with. 2. using a fake round is ugly
				// s.driverPrecommitListener.Receive(ctx,) // Todo: send in all precommit votes, which we don't have.
				// s.driverProposalListener.Receive(ctx,) // Todo: build proposal

				s.curHeight.Add(1)
			}
		}
	}
}

func (s *Service[V, H, A]) deleteOldMessages(height types.Height) {
	for hh := range s.messages {
		if hh < height {
			delete(s.messages, hh)
		}
	}
}
