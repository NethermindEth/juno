package proposal

import (
	"fmt"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

type proposalStream struct {
	messages           map[uint64]*consensus.StreamMessage
	transition         Transition
	stateMachine       ProposalStateMachine
	nextSequenceNumber uint64
}

func newSingleProposalStream(transition Transition) starknetProposalStream {
	return &proposalStream{
		messages:           make(map[uint64]*consensus.StreamMessage),
		transition:         transition,
		stateMachine:       transition.InitialState(),
		nextSequenceNumber: 0,
	}
}

func (s *proposalStream) OnStreamMessage(streamMessage *consensus.StreamMessage) (*starknet.Proposal, error) {
	if s.nextSequenceNumber != streamMessage.SequenceNumber {
		s.messages[streamMessage.SequenceNumber] = streamMessage
		return nil, nil
	}

	exists := true
	for exists {
		switch msg := streamMessage.GetMessage().(type) {
		case *consensus.StreamMessage_Content:
			proposalPart := &consensus.ProposalPart{}
			var err error
			if err = proto.Unmarshal(msg.Content, proposalPart); err != nil {
				return nil, err
			}

			if s.stateMachine, err = s.stateMachine.OnEvent(s.transition, proposalPart); err != nil {
				return nil, err
			}

			s.nextSequenceNumber++
			streamMessage, exists = s.messages[s.nextSequenceNumber]
			delete(s.messages, s.nextSequenceNumber)
		case *consensus.StreamMessage_Fin:
			switch finState := s.stateMachine.(type) {
			case *FinState:
				return (*starknet.Proposal)(finState), nil
			default:
				return nil, fmt.Errorf("stream ended with unexpected state")
			}
		default:
			return nil, fmt.Errorf("unknown message type")
		}
	}
	return nil, nil
}
