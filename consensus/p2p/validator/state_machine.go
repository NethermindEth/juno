package validator

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/adapters/p2p2consensus"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

// TODO: better error handling
var errInvalidMessage = errors.New("invalid message")

type ProposalStateMachine interface {
	OnEvent(context.Context, Transition, *consensus.ProposalPart) (ProposalStateMachine, error)
}

// InitialState handles the first step in the proposal flow,
// accepting only a ProposalInit message to begin the process.
type InitialState struct{}

func (s *InitialState) OnEvent(
	ctx context.Context,
	transition Transition,
	part *consensus.ProposalPart,
) (ProposalStateMachine, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Init:
		proposalInit, err := p2p2consensus.AdaptProposalInit(part.Init)
		if err != nil {
			return nil, err
		}
		return transition.OnProposalInit(ctx, s, &proposalInit)
	default:
		return nil, errInvalidMessage
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
) (ProposalStateMachine, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_BlockInfo:
		blockInfo, err := p2p2consensus.AdaptBlockInfo(part.BlockInfo)
		if err != nil {
			return nil, err
		}
		return transition.OnBlockInfo(ctx, s, &blockInfo)
	case *consensus.ProposalPart_Commitment:
		proposalCommitment, err := p2p2consensus.AdaptProposalCommitment(part.Commitment)
		if err != nil {
			return nil, err
		}
		return transition.OnEmptyBlockCommitment(ctx, s, &proposalCommitment)
	default:
		return nil, errInvalidMessage
	}
}

// ReceivingTransactionsState handles the flow where transaction batches are received,
// continuing until a ProposalCommitment message is received to conclude the input.
type ReceivingTransactionsState struct {
	Header     *starknet.MessageHeader
	ValidRound types.Round
	BuildState *builder.BuildState
}

func (s *ReceivingTransactionsState) OnEvent(
	ctx context.Context,
	transition Transition,
	part *consensus.ProposalPart,
) (ProposalStateMachine, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Transactions:
		transactions, err := p2p2consensus.AdaptProposalTransaction(
			ctx, transition.Compiler(), part.Transactions, transition.Network(),
		)
		if err != nil {
			return nil, err
		}
		return transition.OnTransactions(ctx, s, transactions)
	case *consensus.ProposalPart_Commitment:
		proposalCommitment, err := p2p2consensus.AdaptProposalCommitment(part.Commitment)
		if err != nil {
			return nil, err
		}
		return transition.OnProposalCommitment(ctx, s, &proposalCommitment)
	default:
		return nil, errInvalidMessage
	}
}

// AwaitingProposalFinState handles the final phase of the proposal flow,
// waiting for a ProposalFin message that commits the hash of the proposed value.
type AwaitingProposalFinState struct {
	Proposal    *starknet.Proposal
	BuildResult *builder.BuildResult
}

func (s *AwaitingProposalFinState) OnEvent(
	ctx context.Context,
	transition Transition,
	part *consensus.ProposalPart,
) (ProposalStateMachine, error) {
	switch part := part.GetMessages().(type) {
	case *consensus.ProposalPart_Fin:
		proposalFin, err := p2p2consensus.AdaptProposalFin(part.Fin)
		if err != nil {
			return nil, err
		}
		return transition.OnProposalFin(ctx, s, &proposalFin)
	default:
		return nil, errInvalidMessage
	}
}

// FinState contains the output of the proposal flow.
type FinState struct {
	Proposal    *starknet.Proposal
	BuildResult *builder.BuildResult
}

func (s *FinState) OnEvent(
	ctx context.Context,
	transition Transition,
	part *consensus.ProposalPart,
) (ProposalStateMachine, error) {
	return nil, errInvalidMessage
}
