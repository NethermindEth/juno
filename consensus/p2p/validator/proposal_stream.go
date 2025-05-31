package validator

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

type proposalStream struct {
	messages           map[uint64]*consensus.StreamMessage
	transition         Transition
	stateMachine       ProposalStateMachine
	executionResult    ExecutionResult
	nextSequenceNumber uint64
}

func newSingleProposalStream(transition Transition) *proposalStream {
	return &proposalStream{
		messages:           make(map[uint64]*consensus.StreamMessage),
		transition:         transition,
		stateMachine:       &InitialState{},
		nextSequenceNumber: 0,
	}
}

func (s *proposalStream) OnStreamMessage(ctx context.Context, streamMessage *consensus.StreamMessage) (*starknet.Proposal, error) {
	if s.nextSequenceNumber != streamMessage.SequenceNumber {
		s.messages[streamMessage.SequenceNumber] = streamMessage
		return nil, nil
	}

	exists := true
	for exists {
		switch msg := streamMessage.GetMessage().(type) {
		case *consensus.StreamMessage_Content:
			var err error
			proposalPart := consensus.ProposalPart{}
			if err = proto.Unmarshal(msg.Content, &proposalPart); err != nil {
				return nil, err
			}

			if s.stateMachine, s.executionResult, err = s.stateMachine.OnEvent(ctx, s.transition, &proposalPart); err != nil {
				return nil, err
			}

			s.nextSequenceNumber++
			streamMessage, exists = s.messages[s.nextSequenceNumber]
			delete(s.messages, s.nextSequenceNumber)
		case *consensus.StreamMessage_Fin:
			return s.executionResult.ProposalOutput, nil
		default:
			return nil, fmt.Errorf("unknown message type")
		}
	}
	return nil, nil
}
