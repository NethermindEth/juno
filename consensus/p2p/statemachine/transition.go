package statemachine

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/adapters/p2p2consensus"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/validator"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
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

type transition[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	validator validator.Validator[V, H, A]
}

func NewTransition[V types.Hashable[H], H types.Hash, A types.Addr](
	val validator.Validator[V, H, A],
) Transition {
	return &transition[V, H, A]{
		validator: val,
	}
}

func (t *transition[V, H, A]) OnProposalInit(
	ctx context.Context,
	state *InitialState,
	init *consensus.ProposalInit,
) (*AwaitingBlockInfoOrCommitmentState, error) {
	fmt.Println(" -- 0 OnProposalInit")
	validRound := types.Round(-1)
	if init.ValidRound != nil {
		validRound = types.Round(*init.ValidRound)
	}
	fmt.Println(" -- 1")
	adaptedProposalInit, err := p2p2consensus.AdaptProposalInit(init)
	if err != nil {
		fmt.Println(" -- 1", err)
		return nil, err
	}
	fmt.Println(" -- 2 ")
	if err = t.validator.ProposalInit(&adaptedProposalInit); err != nil {
		fmt.Println(" -- 2", err)
		return nil, err
	}
	fmt.Println(" -- 3 ")
	qwe := &AwaitingBlockInfoOrCommitmentState{
		Header: &starknet.MessageHeader{
			Height: types.Height(init.BlockNumber),
			Round:  types.Round(init.Round),
			Sender: starknet.Address(felt.FromBytes(init.Proposer.Elements)),
		},
		ValidRound: validRound,
	}
	fmt.Println(" -- 4 ")
	return qwe, nil
}

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

func (t *transition[V, H, A]) OnBlockInfo(
	ctx context.Context,
	state *AwaitingBlockInfoOrCommitmentState,
	blockInfo *consensus.BlockInfo,
) (*ReceivingTransactionsState, error) {
	adaptedBlockInfo, err := p2p2consensus.AdaptBlockInfo(blockInfo)
	if err != nil {
		return nil, err
	}
	t.validator.BlockInfo(&adaptedBlockInfo)

	return &ReceivingTransactionsState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      utils.HeapPtr(starknet.Value(blockInfo.Timestamp)),
	}, nil
}

func (t *transition[V, H, A]) OnTransactions(
	ctx context.Context,
	state *ReceivingTransactionsState,
	transactions []*consensus.ConsensusTransaction,
) (*ReceivingTransactionsState, error) {
	txns := make([]types.Transaction, len(transactions))
	for i := range transactions {
		txn, class, err := p2p2consensus.AdaptTransaction(transactions[i])
		if err != nil {
			return nil, err
		}
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

func (t *transition[V, H, A]) OnProposalCommitment(
	ctx context.Context,
	state *ReceivingTransactionsState,
	commitment *consensus.ProposalCommitment,
) (*AwaitingProposalFinState, error) {
	adaptedCommitment, err := p2p2consensus.AdaptProposalCommitment(commitment)
	if err != nil {
		return nil, err
	}

	if err = t.validator.ProposalCommitment(&adaptedCommitment); err != nil {
		return nil, err
	}

	return &AwaitingProposalFinState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Value:      state.Value,
	}, nil
}

func (t *transition[V, H, A]) OnProposalFin(
	ctx context.Context,
	state *AwaitingProposalFinState,
	fin *consensus.ProposalFin,
) (*FinState, error) {
	adaptedFin, err := p2p2consensus.AdaptProposalFin(fin)
	if err != nil {
		return nil, err
	}

	if err = t.validator.ProposalFin(adaptedFin); err != nil {
		return nil, err
	}

	// Todo: This is a hack until we update the starknet.Value.
	// The tests specify starknet.Value as uint64, but we must return a felt in production.
	// For now, I just return the value as a uint64.
	b := fin.ProposalCommitment.Elements
	val := binary.BigEndian.Uint64(b[len(b)-8:])
	nonnilValue := starknet.Value(val)
	return &FinState{
		MessageHeader: *state.Header,
		ValidRound:    state.ValidRound,
		Value:         &nonnilValue,
	}, nil
}
