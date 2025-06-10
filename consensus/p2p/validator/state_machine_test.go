package validator_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/p2p/validator"
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
	postState validator.ProposalStateMachine
}

func TestProposalStateMachine_ValidTranstions(t *testing.T) {
	height := uint64(1337)
	round := uint32(1338)
	validRound := types.Round(-1)
	proposer := validator.GetRandomAddress(t)

	expectedHeader := &starknet.MessageHeader{
		Height: types.Height(height),
		Round:  types.Round(round),
		Sender: starknet.Address(*new(felt.Felt).SetBytes(proposer.Elements)),
	}
	builderAddr := common.Address{Elements: []byte{1}}
	someUint128 := &common.Uint128{Low: 1, High: 2}
	t.Run("valid proposal stream", func(t *testing.T) {
		value := uint64(1339)
		starknetValue := starknet.Value(*new(felt.Felt).SetUint64(value))
		expectedHash := &common.Hash{Elements: validator.ToBytes(*new(felt.Felt).SetUint64(value))}
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
				postState: &validator.AwaitingBlockInfoOrCommitmentState{
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
				postState: &validator.ReceivingTransactionsState{
					Header:     expectedHeader,
					ValidRound: validRound,
					Value:      &starknetValue,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Transactions{Transactions: &consensus.TransactionBatch{
						Transactions: validator.GetRandomTransactions(t, 4),
					}},
				},
				postState: &validator.ReceivingTransactionsState{
					Header:     expectedHeader,
					ValidRound: validRound,
					Value:      &starknetValue,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Transactions{Transactions: &consensus.TransactionBatch{
						Transactions: validator.GetRandomTransactions(t, 5),
					}},
				},
				postState: &validator.ReceivingTransactionsState{
					Header:     expectedHeader,
					ValidRound: validRound,
					Value:      &starknetValue,
				},
			},
			{
				event: &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Commitment{
						Commitment: getExampleProposalCommitment(t),
					},
				},
				postState: &validator.AwaitingProposalFinState{
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
				postState: (*validator.FinState)(expectedProposal),
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
				postState: &validator.AwaitingBlockInfoOrCommitmentState{
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
				postState: &validator.AwaitingProposalFinState{
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
				postState: (*validator.FinState)(expectedProposal),
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
	transition := validator.NewTransition[value, felt.Felt, felt.Felt](mockedValidator)
	var state validator.ProposalStateMachine = &validator.InitialState{}

	for _, step := range steps {
		t.Run(fmt.Sprintf("apply %T to get %T", step.event.Messages, step.postState), func(t *testing.T) {
			state, err = state.OnEvent(t.Context(), transition, step.event)
			require.NoError(t, err)
			assert.Equal(t, step.postState, state)
		})
	}
}

type invalidTransitionTestCase struct {
	state  validator.ProposalStateMachine
	events []*consensus.ProposalPart
}

func TestProposalStateMachine_InvalidTransitions(t *testing.T) {
	steps := []invalidTransitionTestCase{
		{
			state: &validator.InitialState{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Commitment{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
		{
			state: &validator.AwaitingBlockInfoOrCommitmentState{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
		{
			state: &validator.ReceivingTransactionsState{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Fin{}},
			},
		},
		{
			state: &validator.AwaitingProposalFinState{},
			events: []*consensus.ProposalPart{
				{Messages: &consensus.ProposalPart_Init{}},
				{Messages: &consensus.ProposalPart_BlockInfo{}},
				{Messages: &consensus.ProposalPart_Transactions{}},
				{Messages: &consensus.ProposalPart_Commitment{}},
			},
		},
		{
			state: &validator.FinState{},
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
	transition := validator.NewTransition[value, felt.Felt, felt.Felt](mockedValidator)
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
