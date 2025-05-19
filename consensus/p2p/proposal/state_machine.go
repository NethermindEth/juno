package proposal

import (
	"errors"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

// TODO: better error handling
var errInvalidMessage = errors.New("invalid message")

type ProposalStateMachine interface {
	OnEvent(Transition, *consensus.ProposalPart) (ProposalStateMachine, error)
}

type InitialState struct{}

func (s *InitialState) OnEvent(transition Transition, part *consensus.ProposalPart) (ProposalStateMachine, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Init:
		return transition.OnInit(s, part.Init)
	default:
		return nil, errInvalidMessage
	}
}

type ReceivingBlockInfoState struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
}

func (s *ReceivingBlockInfoState) OnEvent(transition Transition, part *consensus.ProposalPart) (ProposalStateMachine, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_BlockInfo:
		return transition.OnBlockInfo(s, part.BlockInfo)
	case *consensus.ProposalPart_Commitment:
		return transition.OnEmptyBlock(s, part.Commitment)
	default:
		return nil, errInvalidMessage
	}
}

type ReceivingTransactionsState struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
	Value      *starknet.Value
}

func (s *ReceivingTransactionsState) OnEvent(transition Transition, part *consensus.ProposalPart) (ProposalStateMachine, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Transactions:
		return transition.OnTransactions(s, part.Transactions.Transactions)
	case *consensus.ProposalPart_Commitment:
		return transition.OnCommitment(s, part.Commitment)
	default:
		return nil, errInvalidMessage
	}
}

type ReceivingHashState struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
	Value      *starknet.Value
}

func (s *ReceivingHashState) OnEvent(transition Transition, part *consensus.ProposalPart) (ProposalStateMachine, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Fin:
		return transition.OnFin(s, part.Fin)
	default:
		return nil, errInvalidMessage
	}
}

type FinState starknet.Proposal

func (s *FinState) OnEvent(_ Transition, _ *consensus.ProposalPart) (ProposalStateMachine, error) {
	return nil, errInvalidMessage
}
