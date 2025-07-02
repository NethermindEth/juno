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

const twofPlus1 = 3 // Todo

type Service[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	driverSyncListener    chan V                     // Notify the Driver that we have fallen behind, and should commit the provided block
	proposalStore         *proposal.ProposalStore[H] // So the Driver can see the block we need to commit
	syncPrevoteListener   vote.VoteListener[types.Prevote[H, A], V, H, A]
	syncPrecommitListener vote.VoteListener[types.Precommit[H, A], V, H, A]
	p2pSync               p2pSync.Service
	messages              map[types.Height]map[A]bool // height-> peer -> bool
	curHeight             atomic.Uint64
	driverCommitNotifier  chan struct{} // Prevents this service falling behind the State StateMachine
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
		messages:              make(map[types.Height]map[A]bool),
		driverCommitNotifier:  commitNotifier,
		log:                   log,
	}
	srv.curHeight.Store(curHeight)
	return &srv
}

func (s *Service[V, H, A]) Run(ctx context.Context) {
	networkHeightCh := make(chan uint64)

	// 1) Collect votes & push futureHeight updates
	// Todo: Ideally we should subscribe to new headers from peers over P2P, instead of
	// waiting for the network to create 2f+1 messasges at a given height.
	// This will allow us to sync faster, while (hopefully) simplifiying the logic.
	// We currently don't have logic to subscribe to new headers over P2P, so continue with this
	// approach for now.
	go func() {
		defer close(networkHeightCh)
		for {
			select {
			// case msg := <-s.syncPrevoteListener.Listen(): // Todo: actually implement this
			// 	if msg.Height > types.Height(s.curHeight.Load()) {
			// 		s.messages[msg.Height][msg.Sender] = true
			// 	}
			case msg := <-s.syncPrevoteListener.Listen():
				if msg.Height > types.Height(s.curHeight.Load()) {
					s.messages[msg.Height][msg.Sender] = true
				}
			case msg := <-s.syncPrecommitListener.Listen():
				if msg.Height > types.Height(s.curHeight.Load()) {
					s.messages[msg.Height][msg.Sender] = true
				}
			case <-s.driverCommitNotifier:
				// Only progress the currentHeight counter when the StateMachine commits a block
				s.deleteOldMessages(types.Height(s.curHeight.Load()))
				s.curHeight.Add(1)
			case <-ctx.Done():
				return
			}

			// once we see > f+1 at some height, push it
			for height, senders := range s.messages {
				if len(senders) > twofPlus1 {
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
