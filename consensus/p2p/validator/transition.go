package validator

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

var (
	errNilProposer  = errors.New("proposer is nil")
	errHashMismatch = errors.New("hash mismatch")
)

type Transition interface {
	OnProposalInit(
		context.Context,
		*InitialState,
		*consensus.ProposalInit,
	) (*AwaitingBlockInfoOrCommitmentState, error)
	OnEmptyBlockCommitment(
		context.Context,
		*AwaitingBlockInfoOrCommitmentState,
		*consensus.ProposalCommitment,
	) (*AwaitingProposalFinState, error)
	OnBlockInfo(
		context.Context,
		*AwaitingBlockInfoOrCommitmentState,
		*consensus.BlockInfo,
	) (*ReceivingTransactionsState, error)
	OnTransactions(
		context.Context,
		*ReceivingTransactionsState,
		[]*consensus.ConsensusTransaction,
	) (*ReceivingTransactionsState, error)
	OnProposalCommitment(
		context.Context,
		*ReceivingTransactionsState,
		*consensus.ProposalCommitment,
	) (*AwaitingProposalFinState, error)
	OnProposalFin(
		context.Context,
		*AwaitingProposalFinState,
		*consensus.ProposalFin,
	) (*FinState, error)
}

type transition struct{}

func NewTransition() Transition {
	return &transition{}
}

// TODO: Implement this function properly
func (t *transition) OnProposalInit(
	ctx context.Context,
	state *InitialState,
	init *consensus.ProposalInit,
) (*AwaitingBlockInfoOrCommitmentState, error) {
	validRound := types.Round(-1)
	if init.ValidRound != nil {
		validRound = types.Round(*init.ValidRound)
	}

	if init.Proposer == nil {
		return nil, errNilProposer
	}
	sender := starknet.Address(felt.FromBytes(init.Proposer.Elements))

	return &AwaitingBlockInfoOrCommitmentState{
		Header: &starknet.MessageHeader{
			Height: types.Height(init.BlockNumber),
			Round:  types.Round(init.Round),
			Sender: sender,
		},
		ValidRound: validRound,
	}, nil
}

// TODO: Implement this function properly
func (t *transition) OnEmptyBlockCommitment(
	ctx context.Context,
	state *AwaitingBlockInfoOrCommitmentState,
	commitment *consensus.ProposalCommitment,
) (*AwaitingProposalFinState, error) {
	return &AwaitingProposalFinState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      nil,
	}, nil
}

// TODO: Implement this function properly
func (t *transition) OnBlockInfo(
	ctx context.Context,
	state *AwaitingBlockInfoOrCommitmentState,
	blockInfo *consensus.BlockInfo,
) (*ReceivingTransactionsState, error) {
	return &ReceivingTransactionsState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      utils.HeapPtr(starknet.Value(felt.FromUint64(blockInfo.Timestamp))),
	}, nil
}

// TODO: Implement this function properly
func (t *transition) OnTransactions(
	ctx context.Context,
	state *ReceivingTransactionsState,
	transactions []*consensus.ConsensusTransaction,
) (*ReceivingTransactionsState, error) {
	return &ReceivingTransactionsState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      state.Value,
	}, nil
}

// TODO: Implement this function properly
func (t *transition) OnProposalCommitment(
	ctx context.Context,
	state *ReceivingTransactionsState,
	commitment *consensus.ProposalCommitment,
) (*AwaitingProposalFinState, error) {
	return &AwaitingProposalFinState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      state.Value,
	}, nil
}

// TODO: Implement this function properly
func (t *transition) OnProposalFin(
	ctx context.Context,
	state *AwaitingProposalFinState,
	fin *consensus.ProposalFin,
) (*FinState, error) {
	// Check if expected and actual are both nil or both non-nil
	if (state.Value == nil) != (fin.ProposalCommitment == nil) {
		return nil, errHashMismatch
	}

	// If both are non-nil, check if they are equal
	if state.Value != nil {
		expected := state.Value.Hash()
		actual := starknet.Hash(felt.FromBytes(fin.ProposalCommitment.Elements))
		if expected != actual {
			return nil, errHashMismatch
		}
	}

	return &FinState{
		MessageHeader: *state.Header,
		Value:         state.Value,
		ValidRound:    state.ValidRound,
	}, nil
}
