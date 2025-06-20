package statemachine

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/validator"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

// TODO: better error handling
var errInvalidMessage = errors.New("invalid message")

// The state machine can progress along two distinct paths:
// Path 1: InitialState -> AwaitingBlockInfoOrCommitmentState -> ReceivingTransactionsState -> AwaitingProposalFinState -> FinState
// Path 2: InitialState -> AwaitingBlockInfoOrCommitmentState ->  FinState
type ProposalStateMachine[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	OnEvent(context.Context, Transition[V, H, A], *consensus.ProposalPart) (ProposalStateMachine[V, H, A], error)
}

// InitialState handles the first step in the proposal flow,
// accepting only a ProposalInit message to begin the process.
type InitialState[V types.Hashable[H], H types.Hash, A types.Addr] struct{}

func (s *InitialState[V, H, A]) OnEvent(
	ctx context.Context,
	transition Transition[V, H, A],
	part *consensus.ProposalPart,
) (ProposalStateMachine[V, H, A], error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Init:
		return transition.OnProposalInit(ctx, s, part.Init)
	default:
		return nil, errInvalidMessage
	}
}

// AwaitingBlockInfoOrCommitmentState handles the transition after ProposalInit,
// accepting either a BlockInfo (for full proposals) or a Commitment (for empty blocks).
type AwaitingBlockInfoOrCommitmentState[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
	Validator  validator.Validator[V, H, A]
}

func (s *AwaitingBlockInfoOrCommitmentState[V, H, A]) OnEvent(
	ctx context.Context,
	transition Transition[V, H, A],
	part *consensus.ProposalPart,
) (ProposalStateMachine[V, H, A], error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_BlockInfo:
		return transition.OnBlockInfo(ctx, s, part.BlockInfo)
	case *consensus.ProposalPart_Commitment:
		return transition.OnEmptyBlockCommitment(ctx, s, part.Commitment)
	default:
		return nil, errInvalidMessage
	}
}

// ReceivingTransactionsState handles the flow where transaction batches are received,
// continuing until a ProposalCommitment message is received to conclude the input.
type ReceivingTransactionsState[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
	Validator  validator.Validator[V, H, A]
}

func (s *ReceivingTransactionsState[V, H, A]) OnEvent(
	ctx context.Context,
	transition Transition[V, H, A],
	part *consensus.ProposalPart,
) (ProposalStateMachine[V, H, A], error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Transactions:
		return transition.OnTransactions(ctx, s, part.Transactions.Transactions)
	case *consensus.ProposalPart_Commitment:
		return transition.OnProposalCommitment(ctx, s, part.Commitment)
	default:
		return nil, errInvalidMessage
	}
}

// AwaitingProposalFinState handles the final phase of the proposal flow,
// waiting for a ProposalFin message that commits the hash of the proposed value.
type AwaitingProposalFinState[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
	Validator  validator.Validator[V, H, A]
}

func (s *AwaitingProposalFinState[V, H, A]) OnEvent(
	ctx context.Context,
	transition Transition[V, H, A],
	part *consensus.ProposalPart,
) (ProposalStateMachine[V, H, A], error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Fin:
		return transition.OnProposalFin(ctx, s, part.Fin)
	default:
		return nil, errInvalidMessage
	}
}

// FinState contains the output of the proposal flow.
type FinState[V types.Hashable[H], H types.Hash, A types.Addr] starknet.Proposal

func (s *FinState[V, H, A]) OnEvent(
	ctx context.Context,
	transition Transition[V, H, A],
	part *consensus.ProposalPart,
) (ProposalStateMachine[V, H, A], error) {
	return nil, errInvalidMessage
}
