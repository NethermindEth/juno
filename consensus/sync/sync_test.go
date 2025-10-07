package sync_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	consensusSync "github.com/NethermindEth/juno/consensus/sync"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/consensus/votecounter"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

const (
	nodeCount = 4
	nodeIndex = nodeCount + 1 // This is to guarantee that this node is not the proposer
)

type app struct {
	cur uint64
}

func (a *app) Value() starknet.Value {
	a.cur = (a.cur + 1) % 100
	return felt.FromUint64[starknet.Value](a.cur)
}

func (a *app) Valid(v starknet.Value) bool {
	return true
}

func TestMessageExtractor(t *testing.T) {
	allNodes := consensus.InitMockServices(0, 0, nodeIndex, nodeCount)

	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	messageExtractor := consensusSync.New[starknet.Value, starknet.Hash, starknet.Address](
		allNodes.Validators,
		toValue,
		&proposalStore,
	)

	blockBody, expectedProposal, expectedPrecommits := getTestData(allNodes.Validators)
	actualProposal, actualPrecommits := messageExtractor.Extract(&blockBody)

	t.Run(("Verify stored build result"), func(t *testing.T) {
		buildResult := proposalStore.Get(actualProposal.Value.Hash())
		require.NotEmpty(t, buildResult)
		require.Equal(t, buildResult.Preconfirmed.Block, blockBody.Block)
		require.Equal(t, buildResult.Preconfirmed.StateUpdate, blockBody.StateUpdate)
		require.Equal(t, buildResult.Preconfirmed.NewClasses, blockBody.NewClasses)
		require.Equal(t, buildResult.SimulateResult.BlockCommitments, blockBody.Commitments)
		require.Equal(t, buildResult.L2GasConsumed, blockBody.Block.L2GasConsumed())
	})

	t.Run(("Verify proposal"), func(t *testing.T) {
		require.Equal(t, expectedProposal, actualProposal)
	})

	t.Run(("Verify precommits"), func(t *testing.T) {
		require.Equal(t, expectedPrecommits, actualPrecommits)
	})

	t.Run(("State machine should be able to commit"), func(t *testing.T) {
		logger := utils.NewNopZapLogger()
		nodeAddr := felt.FromUint64[starknet.Address](uint64(nodeCount) + 1)

		stateMachine := tendermint.New[starknet.Value, starknet.Hash, starknet.Address](
			logger,
			nodeAddr,
			&app{},
			allNodes.Validators,
			actualProposal.Height,
		)
		stateMachine.ProcessStart(0)

		resultActions := stateMachine.ProcessSync(&actualProposal, actualPrecommits)
		expectedCommit := actions.Commit[starknet.Value, starknet.Hash, starknet.Address](
			expectedProposal,
		)
		require.Contains(t, resultActions, &expectedCommit)
	})
}

func getTestData(
	validators votecounter.Validators[starknet.Address],
) (sync.BlockBody, starknet.Proposal, []starknet.Precommit) {
	height := types.Height(rand.Uint64N(1000000))
	round := types.Round(rand.IntN(nodeCount))
	blockHash := felt.Random[felt.Felt]()
	proposerAddr := validators.Proposer(height, round)
	blockBody := sync.BlockBody{
		Block: &core.Block{
			Header: &core.Header{
				Hash:             &blockHash,
				ParentHash:       felt.NewRandom[felt.Felt](),
				Number:           uint64(height),
				GlobalStateRoot:  felt.NewRandom[felt.Felt](),
				SequencerAddress: (*felt.Felt)(&proposerAddr),
				TransactionCount: rand.Uint64N(100),
				EventCount:       rand.Uint64N(1000),
				Timestamp:        uint64(time.Now().Unix()),
				ProtocolVersion:  "0.14.0",
				EventsBloom:      &bloom.BloomFilter{},
				L1GasPriceETH:    felt.NewRandom[felt.Felt](),
				Signatures:       make([][]*felt.Felt, 100),
				L1GasPriceSTRK:   felt.NewRandom[felt.Felt](),
				L1DAMode:         core.Blob,
				L1DataGasPrice: &core.GasPrice{
					PriceInWei: felt.NewRandom[felt.Felt](),
					PriceInFri: felt.NewRandom[felt.Felt](),
				},
				L2GasPrice: &core.GasPrice{
					PriceInWei: felt.NewRandom[felt.Felt](),
					PriceInFri: felt.NewRandom[felt.Felt](),
				},
			},
		},
		StateUpdate: &core.StateUpdate{
			BlockHash: &blockHash,
			NewRoot:   felt.NewRandom[felt.Felt](),
			OldRoot:   felt.NewRandom[felt.Felt](),
			StateDiff: utils.HeapPtr(core.EmptyStateDiff()),
		},
		NewClasses: make(map[felt.Felt]core.ClassDefinition),
		Commitments: &core.BlockCommitments{
			TransactionCommitment: felt.NewRandom[felt.Felt](),
			EventCommitment:       felt.NewRandom[felt.Felt](),
			ReceiptCommitment:     felt.NewRandom[felt.Felt](),
			StateDiffCommitment:   felt.NewRandom[felt.Felt](),
		},
	}

	proposal := starknet.Proposal{
		MessageHeader: types.MessageHeader[starknet.Address]{
			Height: height,
			Round:  round,
			Sender: proposerAddr,
		},
		ValidRound: consensusSync.ValidRoundPlaceholder,
		Value:      (*starknet.Value)(&blockHash),
	}

	precommits := []starknet.Precommit{
		{
			MessageHeader: types.MessageHeader[starknet.Address]{
				Height: height,
				Round:  round,
				Sender: consensusSync.SyncProtocolPrecommitSender,
			},
			ID: (*starknet.Hash)(&blockHash),
		},
	}

	return blockBody, proposal, precommits
}

func toValue(in *felt.Felt) starknet.Value {
	return starknet.Value(*in)
}
