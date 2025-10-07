package sync

import (
	"math"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/sync"
)

// TODO: We use this value until the round is added to the spec
const ValidRoundPlaceholder = -1

// TODO: Remove this once we can extract precommits from the sync protocol messages.
var SyncProtocolPrecommitSender = felt.FromUint64[starknet.Address](math.MaxUint64)

type MessageExtractor[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	validators    votecounter.Validators[A]
	toValue       func(*felt.Felt) V
	proposalStore *proposal.ProposalStore[H]
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	validators votecounter.Validators[A],
	toValue func(*felt.Felt) V,
	proposalStore *proposal.ProposalStore[H],
) MessageExtractor[V, H, A] {
	return MessageExtractor[V, H, A]{
		validators:    validators,
		toValue:       toValue,
		proposalStore: proposalStore,
	}
}

// TODO: This is a temporary solution to find the round, because currently the specs do not have
// the round. This is dangerous because it's not guaranteed to find the round.
// We need eventually remove this.
func (s *MessageExtractor[V, H, A]) findRound(height types.Height, sender A) types.Round {
	var round types.Round
	for {
		if s.validators.Proposer(height, round) == sender {
			return round
		}
		round++
	}
}

func (s *MessageExtractor[V, H, A]) Extract(
	committedBlock *sync.BlockBody,
) (types.Proposal[V, H, A], []types.Precommit[H, A]) {
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

	round := s.findRound(
		types.Height(committedBlock.Block.Number),
		A(*committedBlock.Block.SequencerAddress),
	)

	proposal := types.Proposal[V, H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: types.Height(committedBlock.Block.Number),
			Round:  round,
			Sender: A(*committedBlock.Block.SequencerAddress),
		},
		ValidRound: ValidRoundPlaceholder,
		Value:      &msgV,
	}

	precommits := []types.Precommit[H, A]{
		{
			MessageHeader: types.MessageHeader[A]{
				Height: types.Height(committedBlock.Block.Number),
				Round:  round,
				Sender: A(SyncProtocolPrecommitSender),
			},
			ID: &msgH,
		},
	}

	return proposal, precommits
}
