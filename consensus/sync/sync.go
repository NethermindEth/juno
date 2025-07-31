package sync

import (
	"context"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p"
	p2pSync "github.com/NethermindEth/juno/p2p/sync"
	"github.com/NethermindEth/juno/sync"
)

const syncRoundPlaceHolder = -1 // Todo: We use this value until the round is added to the spec

type Sync[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	syncService       p2p.WithBlockCh
	driverProposalCh  chan types.Proposal[V, H, A]
	driverPrecommitCh chan types.Precommit[H, A]
	// Todo: for now we can forge the precommit votes of our peers
	// In practice, this information needs to be exposed by peers.
	getPrecommits func(types.Height) []types.Precommit[H, A]
	toValue       func(*felt.Felt) V
	proposalStore *proposal.ProposalStore[H]
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	syncService p2p.WithBlockCh,
	driverProposalCh chan types.Proposal[V, H, A],
	driverPrecommitCh chan types.Precommit[H, A],
	getPrecommits func(types.Height) []types.Precommit[H, A],
	toValue func(*felt.Felt) V,
	proposalStore *proposal.ProposalStore[H],
	blockCh chan p2pSync.BlockBody,
) Sync[V, H, A] {
	return Sync[V, H, A]{
		syncService:       syncService,
		driverProposalCh:  driverProposalCh,
		driverPrecommitCh: driverPrecommitCh,
		getPrecommits:     getPrecommits,
		toValue:           toValue,
		proposalStore:     proposalStore,
	}
}

func (s *Sync[V, H, A]) Run(originalCtx context.Context) {
	ctx, cancel := context.WithCancel(originalCtx)
	s.syncService.SetListener()
	go func() {
		err := s.syncService.Run(ctx)
		if err != nil {
			cancel()
			return
		}
	}()

	// Todo: this is kind of ugly, but it unblocks the listen() function in the select-case below
	forwardCh := make(chan p2pSync.BlockBody)
	go func() {
		for {
			inner := s.syncService.Listen()
			for blk := range inner {
				forwardCh <- blk
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			cancel()
			return
		case committedBlock := <-forwardCh:

			msgV := s.toValue(committedBlock.Block.Hash)
			msgH := msgV.Hash()
			concatCommitments := core.ConcatCounts(
				committedBlock.Block.TransactionCount,
				committedBlock.Block.EventCount,
				committedBlock.StateUpdate.StateDiff.Length(),
				committedBlock.Block.L1DAMode,
			)
			buildResult := builder.BuildResult{
				Pending: &sync.Pending{
					Block:       committedBlock.Block,
					StateUpdate: committedBlock.StateUpdate,
					NewClasses:  committedBlock.NewClasses,
				},
				SimulateResult: &blockchain.SimulateResult{
					BlockCommitments: committedBlock.Commitments,
					ConcatCount:      concatCommitments,
				},
				// Todo: this needs added to the spec.
				L2GasConsumed: 1,
			}
			s.proposalStore.Store(msgH, &buildResult)

			// Todo: we can optimise this by performing a height
			// check before pushing everything through consensus.
			// ie skip if syncBlock.Height != DB.Height+1
			precommits := s.getPrecommits(types.Height(committedBlock.Block.Number))
			for _, precommit := range precommits {
				select {
				case <-ctx.Done():
					cancel()
					return
				case s.driverPrecommitCh <- precommit:
				}
			}

			proposal := types.Proposal[V, H, A]{
				MessageHeader: types.MessageHeader[A]{
					Height: types.Height(committedBlock.Block.Number),
					Round:  syncRoundPlaceHolder,
					Sender: committedBlock.Block.SequencerAddress.Bits(),
				},
				ValidRound: -1,
				Value:      &msgV,
			}

			select {
			case <-ctx.Done():
				cancel()
				return
			case s.driverProposalCh <- proposal:
			}

		}
	}
}
