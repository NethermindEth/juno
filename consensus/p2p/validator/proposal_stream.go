package validator

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

type proposalStream struct {
	log                utils.Logger
	input              chan *consensus.StreamMessage
	outputs            chan<- starknet.Proposal
	messages           map[uint64]*consensus.StreamMessage
	transition         Transition
	stateMachine       ProposalStateMachine
	nextSequenceNumber uint64
}

func newSingleProposalStream(log utils.Logger, transition Transition, inputBufferSize int, outputs chan<- starknet.Proposal) *proposalStream {
	return &proposalStream{
		log:                log,
		input:              make(chan *consensus.StreamMessage, inputBufferSize),
		outputs:            outputs,
		messages:           make(map[uint64]*consensus.StreamMessage),
		transition:         transition,
		stateMachine:       &InitialState{},
		nextSequenceNumber: 0,
	}
}

func (s *proposalStream) Start(ctx context.Context, firstMessage *consensus.StreamMessage) (types.Height, error) {
	content := firstMessage.GetContent()
	if content == nil {
		return 0, fmt.Errorf("first message is not a proposal part")
	}

	if err := s.onProposalPart(ctx, content); err != nil {
		return 0, err
	}

	switch state := s.stateMachine.(type) {
	case *AwaitingBlockInfoOrCommitmentState:
		s.nextSequenceNumber = 1
		return state.Header.Height, nil
	default:
		return 0, fmt.Errorf("proposal stream is not in a valid state after ProposalInit")
	}
}

func (s *proposalStream) Loop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case streamMessage, ok := <-s.input:
			if !ok {
				return nil
			}
			if err := s.processMessage(ctx, streamMessage); err != nil {
				return err
			}
		}
	}
}

func (s *proposalStream) Send(ctx context.Context, streamMessage *consensus.StreamMessage) {
	select {
	case <-ctx.Done():
		return
	case s.input <- streamMessage:
	}
}

func (s *proposalStream) Close() {
	close(s.input)
}

func (s *proposalStream) processMessage(ctx context.Context, nextMessage *consensus.StreamMessage) error {
	if nextMessage != nil {
		if s.nextSequenceNumber != nextMessage.SequenceNumber {
			s.messages[nextMessage.SequenceNumber] = nextMessage
			return nil
		}
		s.nextSequenceNumber++
	} else {
		nextMessage = s.getNextMessage()
	}

	for nextMessage != nil {
		switch msg := nextMessage.GetMessage().(type) {
		case *consensus.StreamMessage_Content:
			if err := s.onProposalPart(ctx, msg.Content); err != nil {
				return err
			}
			nextMessage = s.getNextMessage()
		case *consensus.StreamMessage_Fin:
			switch state := s.stateMachine.(type) {
			case *FinState:
				select {
				case <-ctx.Done():
					return ctx.Err()
				case s.outputs <- starknet.Proposal(*state):
					return nil
				}
			default:
				return fmt.Errorf("unknown message type")
			}
		default:
			return fmt.Errorf("unknown message type")
		}
	}
	return nil
}

func (s *proposalStream) onProposalPart(ctx context.Context, messageContent []byte) error {
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

func (s *proposalStream) getNextMessage() *consensus.StreamMessage {
	streamMessage, exists := s.messages[s.nextSequenceNumber]
	if !exists {
		return nil
	}

	delete(s.messages, s.nextSequenceNumber)
	s.nextSequenceNumber++
	return streamMessage
}
