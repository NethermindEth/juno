package statemachine

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

// proposalStream receives and processes parts of Starknet proposals streamed from peers.
// Each part is delivered via the input channel, then ordered and validated.
// Once a complete and valid proposal is assembled, it is sent to the caller via the outputs channel.
type proposalStream[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log      utils.Logger
	input    chan *consensus.StreamMessage
	outputs  chan<- starknet.Proposal
	messages map[uint64]*consensus.StreamMessage
	// Every proposal stream shares the same transition
	transition Transition[V, H, A]
	// Each proposal stream gets its own state. The state contains a validator that is unique to it.
	stateMachine       ProposalStateMachine[V, H, A]
	nextSequenceNumber uint64
}

func newSingleProposalStream[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	transition Transition[V, H, A],
	inputBufferSize int,
	outputs chan<- starknet.Proposal,
) *proposalStream[V, H, A] {
	return &proposalStream[V, H, A]{
		log:                log,
		input:              make(chan *consensus.StreamMessage, inputBufferSize),
		outputs:            outputs,
		messages:           make(map[uint64]*consensus.StreamMessage),
		transition:         transition,
		stateMachine:       &InitialState[V, H, A]{},
		nextSequenceNumber: 0,
	}
}

func (s *proposalStream[V, H, A]) start(ctx context.Context, firstMessage *consensus.StreamMessage) (types.Height, error) {
	content := firstMessage.GetContent()
	if content == nil {
		return 0, fmt.Errorf("first message has empty content")
	}

	if err := s.processProposalPart(ctx, content); err != nil {
		return 0, err
	}

	// The state machine should have progressed from InitialState to AwaitingBlockInfoOrCommitmentState
	switch state := s.stateMachine.(type) {
	case *AwaitingBlockInfoOrCommitmentState[V, H, A]:
		s.nextSequenceNumber = 1
		return state.Header.Height, nil
	default:
		return 0, fmt.Errorf("proposal stream is not in a valid state after ProposalInit")
	}
}

func (s *proposalStream[V, H, A]) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case streamMessage := <-s.input:
			if err := s.processMessages(ctx, streamMessage); err != nil {
				s.log.Errorw("error processing message", "err", err)
				return
			}
		}
	}
}

func (s *proposalStream[V, H, A]) enqueueMessage(ctx context.Context, streamMessage *consensus.StreamMessage) {
	select {
	case <-ctx.Done():
		return
	case s.input <- streamMessage:
	}
}

func (s *proposalStream[V, H, A]) close() {
	close(s.input)
}

func (s *proposalStream[V, H, A]) processMessages(ctx context.Context, nextMessage *consensus.StreamMessage) error {
	if s.nextSequenceNumber != nextMessage.SequenceNumber {
		s.messages[nextMessage.SequenceNumber] = nextMessage
		return nil
	}
	s.nextSequenceNumber++

	for nextMessage != nil {
		switch msg := nextMessage.GetMessage().(type) {
		case *consensus.StreamMessage_Content:
			if err := s.processProposalPart(ctx, msg.Content); err != nil {
				return err
			}
			nextMessage = s.getNextMessage()
		case *consensus.StreamMessage_Fin:
			switch state := s.stateMachine.(type) {
			case *FinState[V, H, A]:
				if state == nil {
					return nil
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case s.outputs <- starknet.Proposal(*state):
					return nil
				}
			default:
				return fmt.Errorf("stream does not end with proposal fin")
			}
		default:
			return fmt.Errorf("unknown message type")
		}
	}
	return nil
}

func (s *proposalStream[V, H, A]) processProposalPart(ctx context.Context, messageContent []byte) error {
	var err error
	proposalPart := consensus.ProposalPart{}
	if err = proto.Unmarshal(messageContent, &proposalPart); err != nil {
		return err
	}

	if s.stateMachine, err = s.stateMachine.OnEvent(ctx, s.transition, &proposalPart); err != nil {
		return err
	}

	return nil
}

func (s *proposalStream[V, H, A]) getNextMessage() *consensus.StreamMessage {
	streamMessage, exists := s.messages[s.nextSequenceNumber]
	if !exists {
		return nil
	}

	delete(s.messages, s.nextSequenceNumber)
	s.nextSequenceNumber++
	return streamMessage
}
