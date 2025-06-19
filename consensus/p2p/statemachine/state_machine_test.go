package statemachine_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/p2p/statemachine"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type value uint64

func (v value) Hash() starknet.Hash {
	return starknet.Hash(*new(felt.Felt).SetUint64(uint64(v)))
}

func getExampleProposalCommitment(t *testing.T) *consensus.ProposalCommitment {
	t.Helper()
	someFelt := &common.Felt252{Elements: []byte{1}}
	someHash := &common.Hash{Elements: []byte{1}}
	someU128 := &common.Uint128{Low: 1, High: 2}
	return &consensus.ProposalCommitment{
		BlockNumber:               123456,
		ParentCommitment:          someHash,
		Builder:                   &common.Address{Elements: []byte{0x02}},
		Timestamp:                 1680000000,
		ProtocolVersion:           "0.12.3",
		OldStateRoot:              someHash,
		VersionConstantCommitment: someHash,
		StateDiffCommitment:       someHash,
		TransactionCommitment:     someHash,
		EventCommitment:           someHash,
		ReceiptCommitment:         someHash,
		ConcatenatedCounts:        someFelt,
		L1GasPriceFri:             someU128,
		L1DataGasPriceFri:         someU128,
		L2GasPriceFri:             someU128,
		L2GasUsed:                 someU128,
		NextL2GasPriceFri:         someU128,
		L1DaMode:                  common.L1DataAvailabilityMode_Blob,
	}
}

type validTransitionTestStep struct {
	event     *consensus.ProposalPart
	postState statemachine.ProposalStateMachine[value, starknet.Hash, starknet.Address]
}

func TestProposalStateMachine_ValidTranstions(t *testing.T) {
	height := uint64(1337)
	round := uint32(1338)
	validRound := types.Round(-1)
	proposer := statemachine.GetRandomAddress(t)
	timestamp := uint64(time.Now().Unix())
	expectedHeader := &starknet.MessageHeader{
		Height: types.Height(height),
		Round:  types.Round(round),
		Sender: starknet.Address(*new(felt.Felt).SetBytes(proposer.Elements)),
	}
	builderAddr := common.Address{Elements: []byte{1}}
	someUint128 := &common.Uint128{Low: 1, High: 2}
	finalProposalCommitment := new(felt.Felt).SetUint64(1).Bytes() // Todo
	finalCommonHash := &common.Hash{Elements: finalProposalCommitment[:]}
	t.Run("valid proposal stream", func(t *testing.T) {
		expectedProposal := &starknet.Proposal{
			MessageHeader: *expectedHeader,
			ValidRound:    validRound,
		}

		runTestSteps(t, []validTransitionTestStep{
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Init{Init: &consensus.ProposalInit{
						BlockNumber: height,
						Round:       round,
						Proposer:    proposer,
					}},
				},
				postState: &statemachine.AwaitingBlockInfoOrCommitmentState[value, starknet.Hash, starknet.Address]{
					Header:     expectedHeader,
					ValidRound: validRound,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_BlockInfo{
						BlockInfo: &consensus.BlockInfo{
							Builder:           &builderAddr,
							BlockNumber:       height,
							Timestamp:         timestamp,
							L2GasPriceFri:     someUint128,
							L1GasPriceWei:     someUint128,
							L1DataGasPriceWei: someUint128,
							EthToStrkRate:     someUint128,
							L1DaMode:          common.L1DataAvailabilityMode_Blob,
						},
					},
				},
				postState: &statemachine.ReceivingTransactionsState[value, starknet.Hash, starknet.Address]{
					Header:     expectedHeader,
					ValidRound: validRound,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Transactions{Transactions: &consensus.TransactionBatch{
						Transactions: statemachine.GetRandomTransactions(t, 4),
					}},
				},
				postState: &statemachine.ReceivingTransactionsState[value, starknet.Hash, starknet.Address]{
					Header:     expectedHeader,
					ValidRound: validRound,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Transactions{Transactions: &consensus.TransactionBatch{
						Transactions: statemachine.GetRandomTransactions(t, 5),
					}},
				},
				postState: &statemachine.ReceivingTransactionsState[value, starknet.Hash, starknet.Address]{
					Header:     expectedHeader,
					ValidRound: validRound,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Commitment{
						Commitment: getExampleProposalCommitment(t),
					},
				},
				postState: &statemachine.AwaitingProposalFinState[value, starknet.Hash, starknet.Address]{
					Header:     expectedHeader,
					ValidRound: validRound,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Fin{
						Fin: &consensus.ProposalFin{ProposalCommitment: finalCommonHash},
					},
				},
				postState: (*statemachine.FinState[value, starknet.Hash, starknet.Address])(expectedProposal),
			},
		})
	})

	t.Run("valid empty proposal stream", func(t *testing.T) {
		expectedProposal := &starknet.Proposal{
			MessageHeader: *expectedHeader,
			ValidRound:    validRound,
		}

		runTestSteps(t, []validTransitionTestStep{
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Init{Init: &consensus.ProposalInit{
						BlockNumber: height,
						Round:       round,
						Proposer:    proposer,
					}},
				},
				postState: &statemachine.AwaitingBlockInfoOrCommitmentState[value, starknet.Hash, starknet.Address]{
					Header:     expectedHeader,
					ValidRound: validRound,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Commitment{
						Commitment: &consensus.ProposalCommitment{
							BlockNumber: 1337,
							Timestamp:   1338,
						},
					},
				},
				postState: &statemachine.AwaitingProposalFinState[value, starknet.Hash, starknet.Address]{
					Header:     expectedHeader,
					ValidRound: validRound,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Fin{
						Fin: &consensus.ProposalFin{ProposalCommitment: finalCommonHash}, // Todo
					},
				},
				postState: (*statemachine.FinState[value, starknet.Hash, starknet.Address])(expectedProposal),
			},
		})
	})
}

func runTestSteps(t *testing.T, steps []validTransitionTestStep) {
	var err error

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockedValidator := mocks.NewMockValidator[value, starknet.Hash, starknet.Address](ctrl)
	mockedValidator.EXPECT().ProposalInit(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().BlockInfo(gomock.Any()).AnyTimes()
	mockedValidator.EXPECT().TransactionBatch(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().ProposalCommitment(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().ProposalFin(gomock.Any()).AnyTimes().Return(nil)
	transition := statemachine.NewTransition[value, starknet.Hash, starknet.Address](mockedValidator)
	var state statemachine.ProposalStateMachine[value, starknet.Hash, starknet.Address] = &statemachine.InitialState[value, starknet.Hash, starknet.Address]{}

	for _, step := range steps {
		t.Run(fmt.Sprintf("apply %T to get %T", step.event.Messages, step.postState), func(t *testing.T) {
			state, err = state.OnEvent(t.Context(), transition, step.event)
			require.NoError(t, err)
			assert.Equal(t, step.postState, state)
		})
	}
}

type invalidTransitionTestCase struct {
	state  statemachine.ProposalStateMachine[value, starknet.Hash, starknet.Address]
	events []*consensus.ProposalPart
}

func TestProposalStateMachine_InvalidTransitions(t *testing.T) {
	steps := []invalidTransitionTestCase{
		{
			state: &statemachine.InitialState[value, starknet.Hash, starknet.Address]{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Commitment{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
		{
			state: &statemachine.AwaitingBlockInfoOrCommitmentState[value, starknet.Hash, starknet.Address]{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
		{
			state: &statemachine.ReceivingTransactionsState[value, starknet.Hash, starknet.Address]{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
		{
			state: &statemachine.AwaitingProposalFinState[value, starknet.Hash, starknet.Address]{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Commitment{}},
			},
		},
		{
			state: &statemachine.FinState[value, starknet.Hash, starknet.Address]{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Commitment{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockedValidator := mocks.NewMockValidator[value, starknet.Hash, starknet.Address](ctrl)
	mockedValidator.EXPECT().ProposalInit(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().BlockInfo(gomock.Any()).AnyTimes()
	mockedValidator.EXPECT().TransactionBatch(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().ProposalCommitment(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().ProposalFin(gomock.Any()).AnyTimes().Return(nil)
	transition := statemachine.NewTransition[value, starknet.Hash, starknet.Address](mockedValidator)
	for _, step := range steps {
		t.Run(fmt.Sprintf("State %T", step.state), func(t *testing.T) {
			for _, event := range step.events {
				t.Run(fmt.Sprintf("Event %T", event.Messages), func(t *testing.T) {
					_, err := step.state.OnEvent(t.Context(), transition, event)
					require.Error(t, err)
				})
			}
		})
	}
}
