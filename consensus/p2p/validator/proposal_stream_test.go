package validator

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestProposalStream_Start(t *testing.T) {
	t.Run("valid proposal init", func(t *testing.T) {
		testProposalStreamStart(t, 1337, "", buildMessage(t, 0, &consensus.ProposalPart{
			Messages: &consensus.ProposalPart_Init{Init: &consensus.ProposalInit{
				BlockNumber: 1337,
				Round:       1338,
				Proposer:    &common.Address{Elements: getRandomFelt(t)},
			}},
		}))
	})

	t.Run("error on fin", func(t *testing.T) {
		testProposalStreamStart(t, 0, "first message has empty content", &consensus.StreamMessage{
			Message: &consensus.StreamMessage_Fin{
				Fin: &common.Fin{},
			},
		})
	})

	t.Run("error on proposal part that is not a proposal init", func(t *testing.T) {
		testProposalStreamStart(t, 0, "invalid message", buildMessage(t, 0, &consensus.ProposalPart{
			Messages: &consensus.ProposalPart_Fin{Fin: &consensus.ProposalFin{}},
		}))
	})

	t.Run("error on empty stream message", func(t *testing.T) {
		testProposalStreamStart(t, 0, "first message has empty content", &consensus.StreamMessage{})
	})

	t.Run("error on empty message content", func(t *testing.T) {
		testProposalStreamStart(t, 0, "first message has empty content", &consensus.StreamMessage{
			Message: &consensus.StreamMessage_Content{},
		})
	})

	t.Run("error on invalid message content", func(t *testing.T) {
		testProposalStreamStart(t, 0, "cannot parse invalid wire-format data", &consensus.StreamMessage{
			Message: &consensus.StreamMessage_Content{
				Content: []byte{1, 2, 3},
			},
		})
	})

	t.Run("error on empty proposal part", func(t *testing.T) {
		testProposalStreamStart(t, 0, "invalid message", buildMessage(t, 0, &consensus.ProposalPart{}))
	})
}

func buildMessage(t *testing.T, sequenceNumber uint64, proposalPart *consensus.ProposalPart) *consensus.StreamMessage {
	t.Helper()
	proposalPartBytes, err := proto.Marshal(proposalPart)
	require.NoError(t, err)

	return &consensus.StreamMessage{
		SequenceNumber: sequenceNumber,
		Message: &consensus.StreamMessage_Content{
			Content: proposalPartBytes,
		},
	}
}

func testProposalStreamStart(t *testing.T, expectedHeight types.Height, expectedErrorMsg string, message *consensus.StreamMessage) {
	t.Helper()
	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	stream := newSingleProposalStream(
		utils.NewNopZapLogger(),
		&proposalStore,
		NewTransition(nil, nil),
		0,
		nil,
	)
	height, err := stream.start(t.Context(), message)
	assert.Equal(t, expectedHeight, height)
	if expectedErrorMsg != "" {
		assert.Contains(t, err.Error(), expectedErrorMsg)
	} else {
		assert.NoError(t, err)
		assert.Equal(t, uint64(1), stream.nextSequenceNumber)
	}
}

type stepValidationContext struct {
	t                     *testing.T
	stream                *proposalStream
	err                   error
	message               *consensus.StreamMessage
	oldStateMachine       ProposalStateMachine
	oldNextSequenceNumber uint64
	outputs               <-chan *starknet.Proposal
}

// There are three possible flows:
// 1. The received message can't be processed right now, so it only stored (messageStoredStepResult)
// 2. The received message can be processed, which may result in other stored messages also being processed (streamPostStateStepResult)
// 3. The received message is not expected, so the stream ends with an error (errorStepResult)

type expectedStepResult interface {
	validate(ctx *stepValidationContext)
}

type messageStoredStepResult struct{}

func (e messageStoredStepResult) validate(ctx *stepValidationContext) {
	require.NoError(ctx.t, ctx.err)
	require.Contains(ctx.t, ctx.stream.messages, ctx.message.SequenceNumber, "message not stored")
	assert.Equal(ctx.t, ctx.message, ctx.stream.messages[ctx.message.SequenceNumber], "message not stored correctly")
	assert.Equal(ctx.t, ctx.oldStateMachine, ctx.stream.stateMachine, "state machine changed")
	assert.Equal(ctx.t, ctx.oldNextSequenceNumber, ctx.stream.nextSequenceNumber, "next sequence number changed")
}

type streamPostStateStepResult struct {
	stateMachine       ProposalStateMachine
	nextSequenceNumber uint64
	output             *starknet.Proposal
}

func (e streamPostStateStepResult) validate(ctx *stepValidationContext) {
	require.NoError(ctx.t, ctx.err)
	require.NotContains(ctx.t, ctx.stream.messages, ctx.message.SequenceNumber, "message stored")

	if e.output != nil {
		assertOutput(ctx.t, ctx.outputs, e.output)
	}
	assertNoOutput(ctx.t, ctx.outputs)

	assert.Equal(ctx.t, e.stateMachine, ctx.stream.stateMachine, "state machine not changed correctly")
	assert.Equal(ctx.t, e.nextSequenceNumber, ctx.stream.nextSequenceNumber, "next sequence number not changed correctly")
}

type errorStepResult string

func (e errorStepResult) validate(ctx *stepValidationContext) {
	assert.Contains(ctx.t, ctx.err.Error(), string(e))
}

type step struct {
	message        *consensus.StreamMessage
	expectedResult expectedStepResult
}

func TestProposalStream_ProcessMessage(t *testing.T) {
	testCase := TestCase{
		Height:       1164618,
		Round:        1339,
		ValidRound:   1338,
		Network:      &utils.SepoliaIntegration,
		TxBatchCount: 2,
	}

	t.Run("valid proposal", func(t *testing.T) {
		database := memory.New()
		SetChainHeight(t, database, testCase.Height-1)
		executor := NewMockExecutor(t, testCase.Network)
		fixture := BuildTestFixture(t, executor, database, testCase)

		testProposalStreamProcessMessage(t, fixture.Builder, fixture.ProposalInit, []step{
			{
				message:        buildMessage(t, 3, fixture.Transactions[1]),
				expectedResult: messageStoredStepResult{},
			},
			{
				message: buildMessage(t, 1, fixture.BlockInfo),
				expectedResult: streamPostStateStepResult{
					stateMachine: &ReceivingTransactionsState{
						Header:     &fixture.Proposal.MessageHeader,
						ValidRound: testCase.ValidRound,
						BuildState: fixture.PreState,
					},
					nextSequenceNumber: 2,
				},
			},
			{
				message: buildMessage(t, 2, fixture.Transactions[0]),
				expectedResult: streamPostStateStepResult{
					stateMachine: &ReceivingTransactionsState{
						Header:     &fixture.Proposal.MessageHeader,
						ValidRound: testCase.ValidRound,
						BuildState: fixture.PreState,
					},
					nextSequenceNumber: 4,
				},
			},
			{
				message: &consensus.StreamMessage{
					SequenceNumber: 6,
					Message: &consensus.StreamMessage_Fin{
						Fin: &common.Fin{},
					},
				},
				expectedResult: messageStoredStepResult{},
			},
			{
				message: buildMessage(t, 4, fixture.ProposalCommitment),
				expectedResult: streamPostStateStepResult{
					stateMachine: &AwaitingProposalFinState{
						Proposal:    fixture.Proposal,
						BuildResult: fixture.BuildResult,
					},
					nextSequenceNumber: 5,
				},
			},
			{
				message: buildMessage(t, 5, fixture.ProposalFin),
				expectedResult: streamPostStateStepResult{
					stateMachine: &FinState{
						Proposal:    fixture.Proposal,
						BuildResult: fixture.BuildResult,
					},
					nextSequenceNumber: 7,
					output:             fixture.Proposal,
				},
			},
		})
	})

	t.Run("empty proposal", func(t *testing.T) {
		database := memory.New()
		SetChainHeight(t, database, testCase.Height-1)
		executor := NewMockExecutor(t, testCase.Network)
		fixture := NewEmptyTestFixture(t, executor, database, testCase)

		testProposalStreamProcessMessage(t, fixture.Builder, fixture.ProposalInit, []step{
			{
				message:        buildMessage(t, 2, fixture.ProposalFin),
				expectedResult: messageStoredStepResult{},
			},
			{
				message: buildMessage(t, 1, fixture.ProposalCommitment),
				expectedResult: streamPostStateStepResult{
					stateMachine: &FinState{
						Proposal:    fixture.Proposal,
						BuildResult: fixture.BuildResult,
					},
					nextSequenceNumber: 3,
				},
			},
			{
				message: &consensus.StreamMessage{
					SequenceNumber: 3,
					Message: &consensus.StreamMessage_Fin{
						Fin: &common.Fin{},
					},
				},
				expectedResult: streamPostStateStepResult{
					stateMachine: &FinState{
						Proposal:    fixture.Proposal,
						BuildResult: fixture.BuildResult,
					},
					nextSequenceNumber: 4,
					output:             fixture.Proposal,
				},
			},
		})
	})

	t.Run("invalid cases", func(t *testing.T) {
		proposalInit := &consensus.ProposalPart{
			Messages: &consensus.ProposalPart_Init{
				Init: &consensus.ProposalInit{
					BlockNumber: 1337,
					Round:       1338,
					Proposer:    &common.Address{Elements: getRandomFelt(t)},
				},
			},
		}

		t.Run("not end with proposal fin", func(t *testing.T) {
			testProposalStreamProcessMessage(t, nil, proposalInit, []step{
				{
					message: &consensus.StreamMessage{
						SequenceNumber: 1,
						Message: &consensus.StreamMessage_Fin{
							Fin: &common.Fin{},
						},
					},
					expectedResult: errorStepResult("stream does not end with proposal fin"),
				},
			})
		})

		t.Run("nil message", func(t *testing.T) {
			testProposalStreamProcessMessage(t, nil, proposalInit, []step{
				{
					message: &consensus.StreamMessage{
						SequenceNumber: 1,
					},
					expectedResult: errorStepResult("unknown message type"),
				},
			})
		})
	})
}

func testProposalStreamProcessMessage(t *testing.T, builder *builder.Builder, proposalInit *consensus.ProposalPart, steps []step) {
	outputs := make(chan *starknet.Proposal, 1)
	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	stream := newSingleProposalStream(
		utils.NewNopZapLogger(),
		&proposalStore,
		NewTransition(builder, nil),
		0,
		outputs,
	)
	_, err := stream.start(t.Context(), buildMessage(t, 1, proposalInit))
	require.NoError(t, err)

	for _, step := range steps {
		t.Run(fmt.Sprintf("%d", step.message.SequenceNumber), func(t *testing.T) {
			oldStateMachine := stream.stateMachine
			oldNextSequenceNumber := stream.nextSequenceNumber

			err := stream.processMessages(t.Context(), step.message)

			step.expectedResult.validate(&stepValidationContext{
				t:                     t,
				stream:                stream,
				err:                   err,
				message:               step.message,
				oldStateMachine:       oldStateMachine,
				oldNextSequenceNumber: oldNextSequenceNumber,
				outputs:               outputs,
			})
		})
	}
}

func assertOutput(t *testing.T, outputs <-chan *starknet.Proposal, expectedOutput *starknet.Proposal) {
	t.Helper()
	select {
	case actualOutput := <-outputs:
		assert.Equal(t, expectedOutput, actualOutput)
	default:
		assert.Fail(t, "outputs channel is empty")
	}
}

func assertNoOutput(t *testing.T, outputs <-chan *starknet.Proposal) {
	t.Helper()
	select {
	case <-outputs:
		assert.Fail(t, "outputs channel is not empty")
	default:
		// Expected, do nothing
	}
}
