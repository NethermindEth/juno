package validator

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
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
	stream := newSingleProposalStream(utils.NewNopZapLogger(), NewTransition(), 0, nil)
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
	outputs               <-chan starknet.Proposal
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
		assertOutput(ctx.t, ctx.outputs, *e.output)
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
	height := types.Height(1337)
	round := types.Round(1338)
	validRound := types.Round(1339)
	value := starknet.Value(1340)
	sender := GetRandomAddress(t)
	expectedHeader := &starknet.MessageHeader{
		Height: height,
		Round:  round,
		Sender: starknet.Address(*new(felt.Felt).SetBytes(sender.Elements)),
	}

	t.Run("valid proposal", func(t *testing.T) {
		expectedHash := &common.Hash{Elements: ToBytes(felt.Felt(value.Hash()))}
		expectedProposal := &starknet.Proposal{
			MessageHeader: *expectedHeader,
			ValidRound:    validRound,
			Value:         &value,
		}
		testProposalStreamProcessMessage(t, sender, []step{
			{
				message: buildMessage(t, 3, &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Transactions{Transactions: &consensus.TransactionBatch{
						Transactions: GetRandomTransactions(t, 5),
					}},
				}),
				expectedResult: messageStoredStepResult{},
			},
			{
				message: buildMessage(t, 1, &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_BlockInfo{BlockInfo: &consensus.BlockInfo{
						BlockNumber: uint64(height),
						Timestamp:   uint64(value),
					}},
				}),
				expectedResult: streamPostStateStepResult{
					stateMachine: &ReceivingTransactionsState{
						Header:     expectedHeader,
						ValidRound: validRound,
						Value:      &value,
					},
					nextSequenceNumber: 2,
				},
			},
			{
				message: buildMessage(t, 2, &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Transactions{Transactions: &consensus.TransactionBatch{
						Transactions: GetRandomTransactions(t, 10),
					}},
				}),
				expectedResult: streamPostStateStepResult{
					stateMachine: &ReceivingTransactionsState{
						Header:     expectedHeader,
						ValidRound: validRound,
						Value:      &value,
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
				message: buildMessage(t, 4, &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Commitment{
						Commitment: &consensus.ProposalCommitment{
							BlockNumber: uint64(height),
							Timestamp:   uint64(value),
						},
					},
				}),
				expectedResult: streamPostStateStepResult{
					stateMachine: &AwaitingProposalFinState{
						Header:     expectedHeader,
						ValidRound: validRound,
						Value:      &value,
					},
					nextSequenceNumber: 5,
				},
			},
			{
				message: buildMessage(t, 5, &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Fin{
						Fin: &consensus.ProposalFin{
							ProposalCommitment: expectedHash,
						},
					},
				}),
				expectedResult: streamPostStateStepResult{
					stateMachine:       (*FinState)(expectedProposal),
					nextSequenceNumber: 7,
					output:             expectedProposal,
				},
			},
		})
	})

	t.Run("empty proposal", func(t *testing.T) {
		expectedProposal := &starknet.Proposal{
			MessageHeader: *expectedHeader,
			ValidRound:    validRound,
		}
		testProposalStreamProcessMessage(t, sender, []step{
			{
				message: buildMessage(t, 2, &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Fin{
						Fin: &consensus.ProposalFin{},
					},
				}),
				expectedResult: messageStoredStepResult{},
			},
			{
				message: buildMessage(t, 1, &consensus.ProposalPart{
					Messages: &consensus.ProposalPart_Commitment{
						Commitment: &consensus.ProposalCommitment{
							BlockNumber: 1337,
							Timestamp:   1338,
						},
					},
				}),
				expectedResult: streamPostStateStepResult{
					stateMachine:       (*FinState)(expectedProposal),
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
					stateMachine:       (*FinState)(expectedProposal),
					nextSequenceNumber: 4,
					output:             expectedProposal,
				},
			},
		})
	})

	t.Run("not end with proposal fin", func(t *testing.T) {
		testProposalStreamProcessMessage(t, sender, []step{
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
		testProposalStreamProcessMessage(t, sender, []step{
			{
				message: &consensus.StreamMessage{
					SequenceNumber: 1,
				},
				expectedResult: errorStepResult("unknown message type"),
			},
		})
	})
}

func testProposalStreamProcessMessage(t *testing.T, sender *common.Address, steps []step) {
	outputs := make(chan starknet.Proposal, 1)
	stream := newSingleProposalStream(utils.NewNopZapLogger(), NewTransition(), 0, outputs)
	_, err := stream.start(t.Context(), buildMessage(t, 1, &consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Init{Init: &consensus.ProposalInit{
			BlockNumber: 1337,
			Round:       1338,
			ValidRound:  utils.HeapPtr(uint32(1339)),
			Proposer:    sender,
		}},
	}))
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

func assertOutput(t *testing.T, outputs <-chan starknet.Proposal, expectedOutput starknet.Proposal) {
	t.Helper()
	select {
	case actualOutput := <-outputs:
		assert.Equal(t, expectedOutput, actualOutput)
	default:
		assert.Fail(t, "outputs channel is empty")
	}
}

func assertNoOutput(t *testing.T, outputs <-chan starknet.Proposal) {
	t.Helper()
	select {
	case <-outputs:
		assert.Fail(t, "outputs channel is not empty")
	default:
		// Expected, do nothing
	}
}
