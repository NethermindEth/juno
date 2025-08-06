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
)

const syncRoundPlaceHolder = 0 // Todo: We use this value until the round is added to the spec

type Sync[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	syncService       p2p.BlockListener
	driverProposalCh  chan types.Proposal[V, H, A]
	driverPrecommitCh chan types.Precommit[H, A]
	// Todo: for now we can forge the precommit votes of our peers
	// In practice, this information needs to be exposed by peers.
	getPrecommits func(types.Height) []types.Precommit[H, A]
	toValue       func(*felt.Felt) V
	proposalStore *proposal.ProposalStore[H]
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	syncService p2p.BlockListener,
	driverProposalCh chan types.Proposal[V, H, A],
	driverPrecommitCh chan types.Precommit[H, A],
	getPrecommits func(types.Height) []types.Precommit[H, A],
	toValue func(*felt.Felt) V,
	proposalStore *proposal.ProposalStore[H],
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

func (s *Sync[V, H, A]) Run(originalCtx context.Context) error {
	ctx, cancel := context.WithCancel(originalCtx)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		defer cancel()
		err := s.syncService.Run(ctx)
		errCh <- err
	}()

	for {
		select {
		case <-ctx.Done():
			select {
			case err := <-errCh:
				return err
			default:
				return ctx.Err()
			}

		case err := <-errCh:
			return err

		case committedBlock := <-s.syncService.Listen():
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
				L2GasConsumed: 1, // TODO
			}
			s.proposalStore.Store(msgH, &buildResult)

			precommits := s.getPrecommits(types.Height(committedBlock.Block.Number))
			for _, precommit := range precommits {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case s.driverPrecommitCh <- precommit:
				}
			}

			proposal := types.Proposal[V, H, A]{
				MessageHeader: types.MessageHeader[A]{
					Height: types.Height(committedBlock.Block.Number),
					Round:  syncRoundPlaceHolder,
					Sender: A(committedBlock.Block.SequencerAddress.Bits()),
				},
				ValidRound: -1,
				Value:      &msgV,
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case s.driverProposalCh <- proposal:
			}
		}
	}
}
