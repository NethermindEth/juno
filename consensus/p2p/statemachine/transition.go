package statemachine

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/adapters/p2p2consensus"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/validator"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

var errProposalFinHashMismatch = errors.New("proposal fin hash mismatch")

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

type transition[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	validator validator.Validator[V, H, A]
	network   utils.Network // Todo
}

func NewTransition[V types.Hashable[H], H types.Hash, A types.Addr](
	val validator.Validator[V, H, A],
) Transition {
	return &transition[V, H, A]{
		validator: val,
	}
}

// TODO: Implement this function properly
func (t *transition[V, H, A]) OnProposalInit(
	ctx context.Context,
	state *InitialState,
	init *consensus.ProposalInit,
) (*AwaitingBlockInfoOrCommitmentState, error) {
	validRound := types.Round(-1)
	if init.ValidRound != nil {
		validRound = types.Round(*init.ValidRound)
	}

	if err := validateProposalInit(init); err != nil {
		return nil, err
	}

	adaptedProposalInit := p2p2consensus.AdaptProposalInit(init)
	err := t.validator.ProposalInit(&adaptedProposalInit)
	if err != nil {
		return nil, err
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
func (t *transition[V, H, A]) OnEmptyBlockCommitment(
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
func (t *transition[V, H, A]) OnBlockInfo(
	ctx context.Context,
	state *AwaitingBlockInfoOrCommitmentState,
	blockInfo *consensus.BlockInfo,
) (*ReceivingTransactionsState, error) {
	if err := validateBlockInfo(blockInfo); err != nil {
		return nil, err
	}

	adaptedBlockInfo := p2p2consensus.AdaptBlockInfo(blockInfo)
	t.validator.BlockInfo(&adaptedBlockInfo)

	return &ReceivingTransactionsState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      utils.HeapPtr(starknet.Value(blockInfo.Timestamp)),
	}, nil
}

// TODO: Implement this function properly
func (t *transition[V, H, A]) OnTransactions(
	ctx context.Context,
	state *ReceivingTransactionsState,
	transactions []*consensus.ConsensusTransaction,
) (*ReceivingTransactionsState, error) {
	for _, txn := range transactions {
		if err := validateConsensusTransaction(txn); err != nil {
			return nil, err
		}
	}

	txns := make([]types.Transaction, len(transactions))
	for i := range transactions {
		txn, class := p2p2consensus.AdaptTransaction(transactions[i], &t.network)
		txns[i] = types.Transaction{
			Transaction: txn,
			Class:       class,
		}
	}

	err := t.validator.TransactionBatch(txns)
	if err != nil {
		return nil, err
	}

	return &ReceivingTransactionsState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      state.Value,
	}, nil
}

// TODO: Implement this function properly
func (t *transition[V, H, A]) OnProposalCommitment(
	ctx context.Context,
	state *ReceivingTransactionsState,
	commitment *consensus.ProposalCommitment,
) (*AwaitingProposalFinState, error) {
	if err := validateProposalCommitment(commitment); err != nil {
		return nil, err
	}

	adaptedCommitment := p2p2consensus.AdaptProposalCommitment(commitment)
	err := t.validator.ProposalCommitment(&adaptedCommitment)
	if err != nil {
		return nil, errProposalFinHashMismatch
	}

	return &AwaitingProposalFinState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      state.Value,
	}, nil
}

// TODO: Implement this function properly
func (t *transition[V, H, A]) OnProposalFin(
	ctx context.Context,
	state *AwaitingProposalFinState,
	fin *consensus.ProposalFin,
) (*FinState, error) {
	if err := validateProposalFin(fin); err != nil {
		return nil, err
	}

	finState := &FinState{
		MessageHeader: *state.Header,
		ValidRound:    state.ValidRound,
	}

	adaptedFin := p2p2consensus.AdaptProposalFin(fin)
	err := t.validator.ProposalFin(adaptedFin)
	if err != nil {
		if errors.Is(err, validator.ErrProposalFinMismatch) {
			return finState, nil
		}
		return nil, err
	}

	// Todo: This is a hack until we update the starknet.Value.
	// The tests specify starknet.Value as uint64, but we must return a felt in production.
	// For now, I just return the value as a uint64.
	b := fin.ProposalCommitment.Elements
	val := binary.BigEndian.Uint64(b[len(b)-8:])
	nonnilValue := starknet.Value(val)
	// commitments match, so update the states value
	finState.Value = &nonnilValue
	return finState, nil
}
