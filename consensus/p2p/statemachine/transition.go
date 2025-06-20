package statemachine

import (
	"context"

	"github.com/NethermindEth/juno/adapters/p2p2consensus"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/validator"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

type Transition[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	OnProposalInit(
		context.Context,
		*InitialState[V, H, A],
		*consensus.ProposalInit,
	) (*AwaitingBlockInfoOrCommitmentState[V, H, A], error)
	OnEmptyBlockCommitment(
		context.Context,
		*AwaitingBlockInfoOrCommitmentState[V, H, A],
		*consensus.ProposalCommitment,
	) (*AwaitingProposalFinState[V, H, A], error)
	OnBlockInfo(
		context.Context,
		*AwaitingBlockInfoOrCommitmentState[V, H, A],
		*consensus.BlockInfo,
	) (*ReceivingTransactionsState[V, H, A], error)
	OnTransactions(
		context.Context,
		*ReceivingTransactionsState[V, H, A],
		[]*consensus.ConsensusTransaction,
	) (*ReceivingTransactionsState[V, H, A], error)
	OnProposalCommitment(
		context.Context,
		*ReceivingTransactionsState[V, H, A],
		*consensus.ProposalCommitment,
	) (*AwaitingProposalFinState[V, H, A], error)
	OnProposalFin(
		context.Context,
		*AwaitingProposalFinState[V, H, A],
		*consensus.ProposalFin,
	) (*FinState[V, H, A], error)
}

type transition[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	bc          *blockchain.Blockchain
	vm          vm.VM
	log         utils.Logger
	disableFees bool
}

func NewTransition[V types.Hashable[H], H types.Hash, A types.Addr](
	bc *blockchain.Blockchain,
	vm vm.VM,
	log utils.Logger,
	disableFees bool,
) Transition[V, H, A] {
	return &transition[V, H, A]{
		bc:          bc,
		vm:          vm,
		log:         log,
		disableFees: disableFees,
	}
}

func (t *transition[V, H, A]) OnProposalInit(
	ctx context.Context,
	state *InitialState[V, H, A],
	init *consensus.ProposalInit,
) (*AwaitingBlockInfoOrCommitmentState[V, H, A], error) {
	validRound := types.Round(-1)
	if init.ValidRound != nil {
		validRound = types.Round(*init.ValidRound)
	}

	adaptedProposalInit, err := p2p2consensus.AdaptProposalInit(init)
	if err != nil {
		return nil, err
	}

	newBuilder := builder.New(t.bc, t.vm, t.log, t.disableFees)
	newValidator := validator.New[V, H, A](&newBuilder)
	if err = newValidator.ProposalInit(&adaptedProposalInit); err != nil {
		return nil, err
	}
	return &AwaitingBlockInfoOrCommitmentState[V, H, A]{
		Header: &starknet.MessageHeader{
			Height: types.Height(init.BlockNumber),
			Round:  types.Round(init.Round),
			Sender: starknet.Address(felt.FromBytes(init.Proposer.Elements)),
		},
		ValidRound: validRound,
		Validator:  newValidator,
	}, nil
}

func (t *transition[V, H, A]) OnEmptyBlockCommitment(
	ctx context.Context,
	state *AwaitingBlockInfoOrCommitmentState[V, H, A],
	commitment *consensus.ProposalCommitment,
) (*AwaitingProposalFinState[V, H, A], error) {
	return &AwaitingProposalFinState[V, H, A]{
		Header:     state.Header,
		ValidRound: state.ValidRound,
	}, nil
}

func (t *transition[V, H, A]) OnBlockInfo(
	ctx context.Context,
	state *AwaitingBlockInfoOrCommitmentState[V, H, A],
	blockInfo *consensus.BlockInfo,
) (*ReceivingTransactionsState[V, H, A], error) {
	adaptedBlockInfo, err := p2p2consensus.AdaptBlockInfo(blockInfo)
	if err != nil {
		return nil, err
	}
	state.Validator.BlockInfo(&adaptedBlockInfo)

	return &ReceivingTransactionsState[V, H, A]{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Validator:  state.Validator,
	}, nil
}

func (t *transition[V, H, A]) OnTransactions(
	ctx context.Context,
	state *ReceivingTransactionsState[V, H, A],
	transactions []*consensus.ConsensusTransaction,
) (*ReceivingTransactionsState[V, H, A], error) {
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

	if err := state.Validator.TransactionBatch(txns); err != nil {
		return nil, err
	}

	return &ReceivingTransactionsState[V, H, A]{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Validator:  state.Validator,
	}, nil
}

func (t *transition[V, H, A]) OnProposalCommitment(
	ctx context.Context,
	state *ReceivingTransactionsState[V, H, A],
	commitment *consensus.ProposalCommitment,
) (*AwaitingProposalFinState[V, H, A], error) {
	adaptedCommitment, err := p2p2consensus.AdaptProposalCommitment(commitment)
	if err != nil {
		return nil, err
	}

	if err = state.Validator.ProposalCommitment(&adaptedCommitment); err != nil {
		return nil, err
	}

	return &AwaitingProposalFinState[V, H, A]{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		Validator:  state.Validator,
	}, nil
}

func (t *transition[V, H, A]) OnProposalFin(
	ctx context.Context,
	state *AwaitingProposalFinState[V, H, A],
	fin *consensus.ProposalFin,
) (*FinState[V, H, A], error) {
	adaptedFin, err := p2p2consensus.AdaptProposalFin(fin)
	if err != nil {
		return nil, err
	}
	// The commitments agree
	snValue := starknet.Value(adaptedFin)
	return &FinState[V, H, A]{
		MessageHeader: *state.Header,
		ValidRound:    state.ValidRound,
		Value:         &snValue,
	}, nil
}
