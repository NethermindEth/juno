package validator

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

// TODO: better error handling
var errInvalidMessage = errors.New("invalid message")

type ExecutionResult struct {
	RequiredHeight *types.Height
	ProposalOutput *starknet.Proposal
}

type ProposalStateMachine interface {
	OnEvent(context.Context, Transition, *consensus.ProposalPart) (ProposalStateMachine, ExecutionResult, error)
}

// InitialState handles the first step in the proposal flow,
// accepting only a ProposalInit message to begin the process.
type InitialState struct{}

func (s *InitialState) OnEvent(
	ctx context.Context,
	transition Transition,
	part *consensus.ProposalPart,
) (ProposalStateMachine, ExecutionResult, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Init:
		nextState, err := transition.OnProposalInit(ctx, s, part.Init)
		return nextState, ExecutionResult{RequiredHeight: (*types.Height)(&part.Init.BlockNumber)}, err
	default:
		return nil, ExecutionResult{}, errInvalidMessage
	}
}

// AwaitingBlockInfoOrCommitmentState handles the transition after ProposalInit,
// accepting either a BlockInfo (for full proposals) or a Commitment (for empty blocks).
type AwaitingBlockInfoOrCommitmentState struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
}

func (s *AwaitingBlockInfoOrCommitmentState) OnEvent(
	ctx context.Context,
	transition Transition,
	part *consensus.ProposalPart,
) (ProposalStateMachine, ExecutionResult, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_BlockInfo:
		nextState, err := transition.OnBlockInfo(ctx, s, part.BlockInfo)
		return nextState, ExecutionResult{}, err
	case *consensus.ProposalPart_Commitment:
		nextState, err := transition.OnEmptyBlockCommitment(ctx, s, part.Commitment)
		return nextState, ExecutionResult{}, err
	default:
		return nil, ExecutionResult{}, errInvalidMessage
	}
}

// ReceivingTransactionsState handles the flow where transaction batches are received,
// continuing until a ProposalCommitment message is received to conclude the input.
type ReceivingTransactionsState struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
	Value      *starknet.Value
}

func (s *ReceivingTransactionsState) OnEvent(
	ctx context.Context,
	transition Transition,
	part *consensus.ProposalPart,
) (ProposalStateMachine, ExecutionResult, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Transactions:
		nextState, err := transition.OnTransactions(ctx, s, part.Transactions.Transactions)
		return nextState, ExecutionResult{}, err
	case *consensus.ProposalPart_Commitment:
		nextState, err := transition.OnProposalCommitment(ctx, s, part.Commitment)
		return nextState, ExecutionResult{}, err
	default:
		return nil, ExecutionResult{}, errInvalidMessage
	}
}

// AwaitingProposalFinState handles the final phase of the proposal flow,
// waiting for a ProposalFin message that commits the hash of the proposed value.
type AwaitingProposalFinState struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
	Value      *starknet.Value
}

func (s *AwaitingProposalFinState) OnEvent(
	ctx context.Context,
	transition Transition,
	part *consensus.ProposalPart,
) (ProposalStateMachine, ExecutionResult, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Fin:
		nextState, err := transition.OnProposalFin(ctx, s, part.Fin)
		return nextState, ExecutionResult{ProposalOutput: (*starknet.Proposal)(nextState)}, err
	default:
		return nil, ExecutionResult{}, errInvalidMessage
	}
}

// FinState contains the output of the proposal flow.
type FinState starknet.Proposal

func (s *FinState) OnEvent(
	ctx context.Context,
	transition Transition,
	part *consensus.ProposalPart,
) (ProposalStateMachine, ExecutionResult, error) {
	return nil, ExecutionResult{}, errInvalidMessage
}
