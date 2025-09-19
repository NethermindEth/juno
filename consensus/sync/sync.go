package sync

import (
	"context"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/types/felt"
	"github.com/NethermindEth/juno/p2p/sync"
)

const syncRoundPlaceHolder = 0 // Todo: We use this value until the round is added to the spec

type Sync[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	blockListener     <-chan sync.BlockBody // sync service to be run separately
	driverProposalCh  chan<- *types.Proposal[V, H, A]
	driverPrecommitCh chan<- *types.Precommit[H, A]
	// Todo: for now we can forge the precommit votes of our peers
	// In practice, this information needs to be exposed by peers.
	getPrecommits func(*sync.BlockBody) []types.Precommit[H, A]
	toValue       func(*felt.Felt) V
	proposalStore *proposal.ProposalStore[H]
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	blockListener <-chan sync.BlockBody,
	driverProposalCh chan<- *types.Proposal[V, H, A],
	driverPrecommitCh chan<- *types.Precommit[H, A],
	getPrecommits func(*sync.BlockBody) []types.Precommit[H, A],
	toValue func(*felt.Felt) V,
	proposalStore *proposal.ProposalStore[H],
) Sync[V, H, A] {
	return Sync[V, H, A]{
		blockListener:     blockListener,
		driverProposalCh:  driverProposalCh,
		driverPrecommitCh: driverPrecommitCh,
		getPrecommits:     getPrecommits,
		toValue:           toValue,
		proposalStore:     proposalStore,
	}
}

func (s *Sync[V, H, A]) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case committedBlock, ok := <-s.blockListener:
			if !ok {
				return nil
			}

			msgV := s.toValue(committedBlock.Block.Hash)
			msgH := msgV.Hash()
			concatCommitments := core.ConcatCounts(
				committedBlock.Block.TransactionCount,
				committedBlock.Block.EventCount,
				committedBlock.StateUpdate.StateDiff.Length(),
				committedBlock.Block.L1DAMode,
			)
			buildResult := builder.BuildResult{
				Preconfirmed: &core.PreConfirmed{
					Block:       committedBlock.Block,
					StateUpdate: committedBlock.StateUpdate,
					NewClasses:  committedBlock.NewClasses,
				},
				SimulateResult: &blockchain.SimulateResult{
					BlockCommitments: committedBlock.Commitments,
					ConcatCount:      concatCommitments,
				},
				L2GasConsumed: committedBlock.Block.L2GasConsumed(),
			}
			s.proposalStore.Store(msgH, &buildResult)

			precommits := s.getPrecommits(&committedBlock)
			for _, precommit := range precommits {
				select {
				case <-ctx.Done():
					return nil
				case s.driverPrecommitCh <- &precommit:
				}
			}

			proposal := types.Proposal[V, H, A]{
				MessageHeader: types.MessageHeader[A]{
					Height: types.Height(committedBlock.Block.Number),
					Round:  syncRoundPlaceHolder,
					Sender: A(*committedBlock.Block.SequencerAddress),
				},
				ValidRound: -1,
				Value:      &msgV,
			}

			select {
			case <-ctx.Done():
				return nil
			case s.driverProposalCh <- &proposal:
			}
		}
	}
}
