package validator

import (
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/state_test_utils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

type EmptyTestFixture struct {
	Builder            *builder.Builder
	BuildResult        *builder.BuildResult
	ProposalInit       *consensus.ProposalPart
	ProposalCommitment *consensus.ProposalPart
	ProposalFin        *consensus.ProposalPart
	Proposal           *starknet.Proposal
}

func NewEmptyTestFixture(
	t *testing.T,
	executor *mockExecutor,
	database db.KeyValueStore,
	testCase TestCase,
) EmptyTestFixture {
	headBlock, _ := LoadBlockDependencies(t, database, testCase.Height, testCase.Network)

	proposer := felt.NewRandom[felt.Felt]()
	expectedHash := felt.NewRandom[felt.Felt]()

	timestamp := rand.Uint64()

	buildResult := EmptyBuildResult(headBlock, proposer, expectedHash, timestamp)

	executor.RegisterBuildResult(&buildResult)

	b := builder.New(blockchain.New(
		database,
		testCase.Network,
		statetestutils.UseNewState(),
	), executor)

	proposalCommitment := EmptyProposalCommitment(headBlock, proposer, timestamp)

	expectedHeader := starknet.MessageHeader{
		Height: testCase.Height,
		Round:  testCase.Round,
		Sender: starknet.Address(*proposer),
	}

	expectedProposal := starknet.Proposal{
		MessageHeader: expectedHeader,
		ValidRound:    testCase.ValidRound,
		Value:         (*starknet.Value)(expectedHash),
	}

	proposalInit := &consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Init{
			Init: &consensus.ProposalInit{
				BlockNumber: uint64(testCase.Height),
				Round:       uint32(testCase.Round),
				ValidRound:  utils.HeapPtr(uint32(testCase.ValidRound)),
				Proposer:    &common.Address{Elements: ToBytes(*proposer)},
			},
		},
	}

	proposalFin := &consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Fin{
			Fin: &consensus.ProposalFin{
				ProposalCommitment: &common.Hash{Elements: ToBytes(*expectedHash)},
			},
		},
	}

	return EmptyTestFixture{
		Builder:            &b,
		BuildResult:        &buildResult,
		ProposalInit:       proposalInit,
		ProposalCommitment: proposalCommitment,
		ProposalFin:        proposalFin,
		Proposal:           &expectedProposal,
	}
}

func EmptyBuildResult(headBlock *core.Block, proposer, expectedHash *felt.Felt, timestamp uint64) builder.BuildResult {
	return builder.BuildResult{
		Preconfirmed: &core.PreConfirmed{
			Block: &core.Block{
				Header: &core.Header{
					Hash:             expectedHash,
					ParentHash:       headBlock.Hash,
					Number:           headBlock.Number + 1,
					GlobalStateRoot:  headBlock.GlobalStateRoot,
					SequencerAddress: proposer,
					TransactionCount: 0,
					EventCount:       0,
					Timestamp:        timestamp,
					ProtocolVersion:  builder.CurrentStarknetVersion.String(),
					EventsBloom:      nil,
					L1GasPriceETH:    new(felt.Felt).SetUint64(0),
					Signatures:       nil,
					L1GasPriceSTRK:   new(felt.Felt).SetUint64(0),
					L1DAMode:         core.L1DAMode(0),
					L1DataGasPrice: &core.GasPrice{
						PriceInWei: new(felt.Felt).SetUint64(0),
						PriceInFri: new(felt.Felt).SetUint64(0),
					},
					L2GasPrice: &core.GasPrice{
						PriceInWei: new(felt.Felt).SetUint64(0),
						PriceInFri: new(felt.Felt).SetUint64(0),
					},
				},
				Transactions: []core.Transaction{},
				Receipts:     []*core.TransactionReceipt{},
			},
			StateUpdate: &core.StateUpdate{
				OldRoot:   headBlock.GlobalStateRoot,
				StateDiff: utils.HeapPtr(core.EmptyStateDiff()),
			},
			NewClasses:            make(map[felt.Felt]core.ClassDefinition),
			TransactionStateDiffs: []*core.StateDiff{},
			CandidateTxs:          []core.Transaction{},
		},
		SimulateResult: &blockchain.SimulateResult{
			BlockCommitments: &core.BlockCommitments{
				TransactionCommitment: new(felt.Felt).SetUint64(0),
				EventCommitment:       new(felt.Felt).SetUint64(0),
				ReceiptCommitment:     new(felt.Felt).SetUint64(0),
				StateDiffCommitment:   new(felt.Felt).SetUint64(0),
			},
			ConcatCount: felt.FromUint64[felt.Felt](0),
		},
	}
}

func EmptyProposalCommitment(headBlock *core.Block, proposer *felt.Felt, timestamp uint64) *consensus.ProposalPart {
	return &consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Commitment{
			Commitment: &consensus.ProposalCommitment{
				BlockNumber:               headBlock.Number + 1,
				ParentCommitment:          &common.Hash{Elements: ToBytes(*headBlock.Hash)},
				Builder:                   &common.Address{Elements: ToBytes(*proposer)},
				Timestamp:                 timestamp,
				ProtocolVersion:           builder.CurrentStarknetVersion.String(),
				OldStateRoot:              &common.Hash{Elements: ToBytes(felt.Zero)},
				VersionConstantCommitment: &common.Hash{Elements: ToBytes(felt.Zero)},
				StateDiffCommitment:       &common.Hash{Elements: ToBytes(felt.Zero)},
				TransactionCommitment:     &common.Hash{Elements: ToBytes(felt.Zero)},
				EventCommitment:           &common.Hash{Elements: ToBytes(felt.Zero)},
				ReceiptCommitment:         &common.Hash{Elements: ToBytes(felt.Zero)},
				ConcatenatedCounts:        &common.Felt252{Elements: ToBytes(felt.Zero)},
				L1GasPriceFri:             &common.Uint128{},
				L1DataGasPriceFri:         &common.Uint128{},
				L2GasPriceFri:             &common.Uint128{},
				L2GasUsed:                 &common.Uint128{},
				NextL2GasPriceFri:         &common.Uint128{},
				L1DaMode:                  0,
			},
		},
	}
}
