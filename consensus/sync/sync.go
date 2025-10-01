package sync

import (
	"math"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/sync"
)

const ( // TODO: We use these value until the round is added to the spec
	RoundPlaceholder      = 0
	ValidRoundPlaceholder = -1
)

// TODO: Remove this once we can extract precommits from the sync protocol messages.
var SyncProtocolPrecommitSender = felt.FromUint64[starknet.Address](math.MaxUint64)

type MessageExtractor[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	toValue       func(*felt.Felt) V
	proposalStore *proposal.ProposalStore[H]
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	toValue func(*felt.Felt) V,
	proposalStore *proposal.ProposalStore[H],
) MessageExtractor[V, H, A] {
	return MessageExtractor[V, H, A]{
		toValue:       toValue,
		proposalStore: proposalStore,
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

	proposal := types.Proposal[V, H, A]{
		MessageHeader: types.MessageHeader[A]{
			Height: types.Height(committedBlock.Block.Number),
			Round:  RoundPlaceholder,
			Sender: A(*committedBlock.Block.SequencerAddress),
		},
		ValidRound: ValidRoundPlaceholder,
		Value:      &msgV,
	}

	precommits := []types.Precommit[H, A]{
		{
			MessageHeader: types.MessageHeader[A]{
				Height: types.Height(committedBlock.Block.Number),
				Round:  RoundPlaceholder,
				Sender: A(SyncProtocolPrecommitSender),
			},
			ID: &msgH,
		},
	}

	return proposal, precommits
}
