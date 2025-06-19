package statemachine_test

import (
	"encoding/binary"
	"fmt"
	"testing"

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

func (v value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(v))
}

type validTransitionTestStep struct {
	event     *consensus.ProposalPart
	postState statemachine.ProposalStateMachine
}

func TestProposalStateMachine_ValidTranstions(t *testing.T) {
	height := uint64(1337)
	round := uint32(1338)
	validRound := types.Round(-1)
	proposer := statemachine.GetRandomAddress(t)

	expectedHeader := &starknet.MessageHeader{
		Height: types.Height(height),
		Round:  types.Round(round),
		Sender: starknet.Address(*new(felt.Felt).SetBytes(proposer.Elements)),
	}
	builderAddr := common.Address{Elements: []byte{1}}
	someUint128 := &common.Uint128{Low: 1, High: 2}
	someFelt := &common.Felt252{Elements: []byte{1}}
	someHash := &common.Hash{Elements: []byte{1}}
	t.Run("valid proposal stream", func(t *testing.T) {
		value := uint64(1339)
		starknetValue := starknet.Value(*new(felt.Felt).SetUint64(value))
		expectedHash := &common.Hash{Elements: statemachine.ToBytes(*new(felt.Felt).SetUint64(value))}
		expectedProposal := &starknet.Proposal{
			MessageHeader: *expectedHeader,
			ValidRound:    validRound,
			Value:         &starknetValue,
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
				postState: &statemachine.AwaitingBlockInfoOrCommitmentState{
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
							Timestamp:         value,
							L2GasPriceFri:     someUint128,
							L1GasPriceWei:     someUint128,
							L1DataGasPriceWei: someUint128,
							EthToStrkRate:     someUint128,
							L1DaMode:          common.L1DataAvailabilityMode_Blob,
						},
					},
				},
				postState: &statemachine.ReceivingTransactionsState{
					Header:     expectedHeader,
					ValidRound: validRound,
					Value:      &starknetValue,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Transactions{Transactions: &consensus.TransactionBatch{
						Transactions: statemachine.GetRandomTransactions(t, 4),
					}},
				},
				postState: &statemachine.ReceivingTransactionsState{
					Header:     expectedHeader,
					ValidRound: validRound,
					Value:      &starknetValue,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Transactions{Transactions: &consensus.TransactionBatch{
						Transactions: statemachine.GetRandomTransactions(t, 5),
					}},
				},
				postState: &statemachine.ReceivingTransactionsState{
					Header:     expectedHeader,
					ValidRound: validRound,
					Value:      &starknetValue,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Commitment{
						Commitment: &consensus.ProposalCommitment{
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
							L1GasPriceFri:             someUint128,
							L1DataGasPriceFri:         someUint128,
							L2GasPriceFri:             someUint128,
							L2GasUsed:                 someUint128,
							NextL2GasPriceFri:         someUint128,
							L1DaMode:                  common.L1DataAvailabilityMode_Blob,
						},
					},
				},
				postState: &statemachine.AwaitingProposalFinState{
					Header:     expectedHeader,
					ValidRound: validRound,
					Value:      &starknetValue,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Fin{
						Fin: &consensus.ProposalFin{
							ProposalCommitment: expectedHash,
						},
					},
				},
				postState: (*statemachine.FinState)(expectedProposal),
			},
		})
	})

	t.Run("valid empty proposal stream", func(t *testing.T) {
		value := uint64(1339)
		starknetValue := starknet.Value(*new(felt.Felt).SetUint64(value))
		expectedProposal := &starknet.Proposal{
			MessageHeader: *expectedHeader,
			ValidRound:    validRound,
			Value:         &starknetValue,
		}
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, value)
		validFinCommitment := &common.Hash{Elements: buf}
		runTestSteps(t, []validTransitionTestStep{
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Init{Init: &consensus.ProposalInit{
						BlockNumber: height,
						Round:       round,
						Proposer:    proposer,
					}},
				},
				postState: &statemachine.AwaitingBlockInfoOrCommitmentState{
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
				postState: &statemachine.AwaitingProposalFinState{
					Header:     expectedHeader,
					ValidRound: validRound,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Fin{
						Fin: &consensus.ProposalFin{ProposalCommitment: validFinCommitment},
					},
				},
				postState: (*statemachine.FinState)(expectedProposal),
			},
		})
	})
}

func runTestSteps(t *testing.T, steps []validTransitionTestStep) {
	var err error

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockedValidator := mocks.NewMockValidator[value, felt.Felt, felt.Felt](ctrl)
	mockedValidator.EXPECT().ProposalInit(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().BlockInfo(gomock.Any()).AnyTimes()
	mockedValidator.EXPECT().TransactionBatch(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().ProposalCommitment(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().ProposalFin(gomock.Any()).AnyTimes().Return(nil)
	transition := statemachine.NewTransition[value, felt.Felt, felt.Felt](mockedValidator)
	var state statemachine.ProposalStateMachine = &statemachine.InitialState{}

	for _, step := range steps {
		t.Run(fmt.Sprintf("apply %T to get %T", step.event.Messages, step.postState), func(t *testing.T) {
			state, err = state.OnEvent(t.Context(), transition, step.event)
			require.NoError(t, err)
			assert.Equal(t, step.postState, state)
		})
	}
}

type invalidTransitionTestCase struct {
	state  statemachine.ProposalStateMachine
	events []*consensus.ProposalPart
}

func TestProposalStateMachine_InvalidTransitions(t *testing.T) {
	steps := []invalidTransitionTestCase{
		{
			state: &statemachine.InitialState{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Commitment{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
		{
			state: &statemachine.AwaitingBlockInfoOrCommitmentState{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
		{
			state: &statemachine.ReceivingTransactionsState{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
		{
			state: &statemachine.AwaitingProposalFinState{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Commitment{}},
			},
		},
		{
			state: &statemachine.FinState{},
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
	mockedValidator := mocks.NewMockValidator[value, felt.Felt, felt.Felt](ctrl)
	mockedValidator.EXPECT().ProposalInit(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().BlockInfo(gomock.Any()).AnyTimes()
	mockedValidator.EXPECT().TransactionBatch(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().ProposalCommitment(gomock.Any()).AnyTimes().Return(nil)
	mockedValidator.EXPECT().ProposalFin(gomock.Any()).AnyTimes().Return(nil)
	transition := statemachine.NewTransition[value, felt.Felt, felt.Felt](mockedValidator)
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
