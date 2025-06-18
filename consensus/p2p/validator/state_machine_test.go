package validator_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/consensus/p2p/validator"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	t.Run("valid proposal stream", func(t *testing.T) {
		value := uint64(1339)
		starknetValue := starknet.Value(felt.FromUint64(value))
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
					Messages: &consensus.ProposalPart_BlockInfo{BlockInfo: &consensus.BlockInfo{
						BlockNumber: height,
						Timestamp:   value,
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
						Commitment: &consensus.ProposalCommitment{
							BlockNumber: height,
							Timestamp:   value,
						},
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

	t.Run("empty proposal stream", func(t *testing.T) {
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
						Fin: &consensus.ProposalFin{},
					},
				},
				postState: (*validator.FinState)(expectedProposal),
			},
		})
	})
}

func runTestSteps(t *testing.T, steps []validTransitionTestStep) {
	var err error
	transition := validator.NewTransition()
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

	transition := validator.NewTransition()
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
