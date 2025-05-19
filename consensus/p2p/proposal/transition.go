package proposal

import (
	"errors"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

var (
	errNilProposer           = errors.New("proposer is nil")
	errNilProposalCommitment = errors.New("proposal commitment is nil")
	errHashMismatch          = errors.New("hash mismatch")
)

type Transition interface {
	InitialState() *InitialState
	OnInit(*InitialState, *consensus.ProposalInit) (*ReceivingBlockInfoState, error)
	OnEmptyBlock(*ReceivingBlockInfoState, *consensus.ProposalCommitment) (*ReceivingHashState, error)
	OnBlockInfo(*ReceivingBlockInfoState, *consensus.BlockInfo) (*ReceivingTransactionsState, error)
	OnTransactions(*ReceivingTransactionsState, []*consensus.ConsensusTransaction) (*ReceivingTransactionsState, error)
	OnCommitment(*ReceivingTransactionsState, *consensus.ProposalCommitment) (*ReceivingHashState, error)
	OnFin(*ReceivingHashState, *consensus.ProposalFin) (*FinState, error)
}

type transition struct{}

func NewTransition() Transition {
	return &transition{}
}

func (t *transition) InitialState() *InitialState {
	// TODO: Implement this function
	return &InitialState{}
}

// TODO: Implement this function properly
func (t *transition) OnInit(
	state *InitialState,
	init *consensus.ProposalInit,
) (*ReceivingBlockInfoState, error) {
	validRound := types.Round(-1)
	if init.ValidRound != nil {
		validRound = types.Round(*init.ValidRound)
	}

	if init.Proposer == nil {
		return nil, errNilProposer
	}
	sender := starknet.Address(felt.FromBytes(init.Proposer.Elements))

	return &ReceivingBlockInfoState{
		Header: &starknet.MessageHeader{
			Height: types.Height(init.BlockNumber),
			Round:  types.Round(init.Round),
			Sender: sender,
		},
		ValidRound: validRound,
	}, nil
}

// TODO: Implement this function properly
func (t *transition) OnEmptyBlock(
	state *ReceivingBlockInfoState,
	commitment *consensus.ProposalCommitment,
) (*ReceivingHashState, error) {
	return &ReceivingHashState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      utils.HeapPtr(starknet.Value(0)),
	}, nil
}

// TODO: Implement this function properly
func (t *transition) OnBlockInfo(
	state *ReceivingBlockInfoState,
	blockInfo *consensus.BlockInfo,
) (*ReceivingTransactionsState, error) {
	return &ReceivingTransactionsState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      utils.HeapPtr(starknet.Value(blockInfo.Timestamp)),
	}, nil
}

// TODO: Implement this function properly
func (t *transition) OnTransactions(
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
func (t *transition) OnCommitment(
	state *ReceivingTransactionsState,
	commitment *consensus.ProposalCommitment,
) (*ReceivingHashState, error) {
	return &ReceivingHashState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      state.Value,
	}, nil
}

// TODO: Implement this function properly
func (t *transition) OnFin(
	state *ReceivingHashState,
	fin *consensus.ProposalFin,
) (*FinState, error) {
	if fin.ProposalCommitment == nil {
		return nil, errNilProposalCommitment
	}
	id := starknet.Hash(felt.FromBytes(fin.ProposalCommitment.Elements))

	if state.Value.Hash() != id {
		return nil, errHashMismatch
	}

	return &FinState{
		MessageHeader: *state.Header,
		Value:         state.Value,
		ValidRound:    state.ValidRound,
	}, nil
}
