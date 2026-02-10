package validator_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/p2p/validator"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type validTransitionTestStep struct {
	events    []*consensus.ProposalPart
	postState validator.ProposalStateMachine
}

func TestProposalStateMachine_EmptyProposalStream(t *testing.T) {
	testCase := validator.TestCase{
		Height:     1164618,
		Round:      2,
		ValidRound: 1,
		Network:    &utils.SepoliaIntegration,
	}

	database := memory.New()
	validator.SetChainHeight(t, database, testCase.Height-1)
	executor := validator.NewMockExecutor(t, testCase.Network)
	fixture := validator.NewEmptyTestFixture(t, executor, database, testCase)

	runTestSteps(t, fixture.Builder, []validTransitionTestStep{
		{
			events: []*consensus.ProposalPart{fixture.ProposalInit},
			postState: &validator.AwaitingBlockInfoOrCommitmentState{
				Header:     &fixture.Proposal.MessageHeader,
				ValidRound: testCase.ValidRound,
			},
		},
		{
			events: []*consensus.ProposalPart{fixture.ProposalCommitment},
			postState: &validator.AwaitingProposalFinState{
				Proposal:    fixture.Proposal,
				BuildResult: fixture.BuildResult,
			},
		},
		{
			events: []*consensus.ProposalPart{fixture.ProposalFin},
			postState: &validator.FinState{
				Proposal:    fixture.Proposal,
				BuildResult: fixture.BuildResult,
			},
		},
	})
}

func TestProposalStateMachine_NonEmptyProposalStream(t *testing.T) {
	testCases := []validator.TestCase{
		{
			Height:       1164618,
			Round:        2,
			ValidRound:   -1,
			Network:      &utils.SepoliaIntegration,
			TxBatchCount: 3,
		},
	}
	for _, testCase := range testCases {
		runNonEmptyProposalStream(t, testCase)
	}
}

func runNonEmptyProposalStream(t *testing.T, testCase validator.TestCase) {
	t.Run(fmt.Sprintf("%v-%v", testCase.Network.Name, testCase.Height), func(t *testing.T) {
		database := memory.New()
		validator.SetChainHeight(t, database, testCase.Height-1)
		executor := validator.NewMockExecutor(t, testCase.Network)
		fixture := validator.BuildTestFixture(t, executor, database, testCase)

		runTestSteps(t, fixture.Builder, []validTransitionTestStep{
			{
				events: []*consensus.ProposalPart{fixture.ProposalInit},
				postState: &validator.AwaitingBlockInfoOrCommitmentState{
					Header:     &fixture.Proposal.MessageHeader,
					ValidRound: testCase.ValidRound,
				},
			},
			{
				events: []*consensus.ProposalPart{fixture.BlockInfo},
				postState: &validator.ReceivingTransactionsState{
					Header:     &fixture.Proposal.MessageHeader,
					ValidRound: testCase.ValidRound,
					BuildState: fixture.PreState,
				},
			},
			{
				events: fixture.Transactions,
				postState: &validator.ReceivingTransactionsState{
					Header:     &fixture.Proposal.MessageHeader,
					ValidRound: testCase.ValidRound,
					BuildState: fixture.PreState,
				},
			},
			{
				events: []*consensus.ProposalPart{fixture.ProposalCommitment},
				postState: &validator.AwaitingProposalFinState{
					Proposal:    fixture.Proposal,
					BuildResult: fixture.BuildResult,
				},
			},
			{
				events: []*consensus.ProposalPart{fixture.ProposalFin},
				postState: &validator.FinState{
					Proposal:    fixture.Proposal,
					BuildResult: fixture.BuildResult,
				},
			},
		})
	})
}

func runTestSteps(t *testing.T, builder *builder.Builder, steps []validTransitionTestStep) {
	var err error
	transition := validator.NewTransition(builder, nil)
	var state validator.ProposalStateMachine = &validator.InitialState{}

	for _, step := range steps {
		for _, event := range step.events {
			t.Run(fmt.Sprintf("applying %T", event.Messages), func(t *testing.T) {
				state, err = state.OnEvent(t.Context(), transition, event)
				require.NoError(t, err)
			})
		}
		t.Run(fmt.Sprintf("validating state %T", state), func(t *testing.T) {
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

	transition := validator.NewTransition(nil, nil)
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
