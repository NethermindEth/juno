package validator

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
)

var errHashMismatch = errors.New("hash mismatch")

type Transition interface {
	Network() *utils.Network
	Compiler() compiler.Compiler
	OnProposalInit(
		context.Context,
		*InitialState,
		*types.ProposalInit,
	) (*AwaitingBlockInfoOrCommitmentState, error)
	OnEmptyBlockCommitment(
		context.Context,
		*AwaitingBlockInfoOrCommitmentState,
		*types.ProposalCommitment,
	) (*AwaitingProposalFinState, error)
	OnBlockInfo(
		context.Context,
		*AwaitingBlockInfoOrCommitmentState,
		*types.BlockInfo,
	) (*ReceivingTransactionsState, error)
	OnTransactions(
		context.Context,
		*ReceivingTransactionsState,
		[]types.Transaction,
	) (*ReceivingTransactionsState, error)
	OnProposalCommitment(
		context.Context,
		*ReceivingTransactionsState,
		*types.ProposalCommitment,
	) (*AwaitingProposalFinState, error)
	OnProposalFin(
		context.Context,
		*AwaitingProposalFinState,
		*types.ProposalFin,
	) (*FinState, error)
}

type transition struct {
	builder  *builder.Builder
	compiler compiler.Compiler
}

func NewTransition(
	builder *builder.Builder, compiler compiler.Compiler,
) Transition {
	return &transition{
		builder:  builder,
		compiler: compiler,
	}
}

func (t *transition) Network() *utils.Network {
	return t.builder.Network()
}

func (t *transition) Compiler() compiler.Compiler {
	return t.compiler
}

func (t *transition) OnProposalInit(
	ctx context.Context,
	state *InitialState,
	init *types.ProposalInit,
) (*AwaitingBlockInfoOrCommitmentState, error) {
	return &AwaitingBlockInfoOrCommitmentState{
		Header: &starknet.MessageHeader{
			Height: init.BlockNum,
			Round:  init.Round,
			Sender: starknet.Address(init.Proposer),
		},
		ValidRound: init.ValidRound,
	}, nil
}

func (t *transition) OnEmptyBlockCommitment(
	ctx context.Context,
	state *AwaitingBlockInfoOrCommitmentState,
	commitment *types.ProposalCommitment,
) (*AwaitingProposalFinState, error) {
	buildState, err := t.builder.InitPreconfirmedBlock(&builder.BuildParams{
		Builder:           felt.Felt(state.Header.Sender),
		Timestamp:         commitment.Timestamp,
		L2GasPriceFRI:     felt.Zero,
		L1GasPriceWEI:     felt.Zero,
		L1DataGasPriceWEI: felt.Zero,
		EthToStrkRate:     felt.Zero,
		L1DAMode:          core.L1DAMode(0), // Use 0 instead of Blob or CallData to explicitly set 0
	})
	if err != nil {
		return nil, err
	}

	buildResult, err := t.finishAndValidateBlock(buildState, commitment)
	if err != nil {
		return nil, err
	}

	return &AwaitingProposalFinState{
		Proposal: &starknet.Proposal{
			MessageHeader: *state.Header,
			Value:         (*starknet.Value)(buildResult.Preconfirmed.Block.Hash),
			ValidRound:    state.ValidRound,
		},
		BuildResult: &buildResult,
	}, nil
}

func (t *transition) OnBlockInfo(
	ctx context.Context,
	state *AwaitingBlockInfoOrCommitmentState,
	blockInfo *types.BlockInfo,
) (*ReceivingTransactionsState, error) {
	buildState, err := t.builder.InitPreconfirmedBlock(&builder.BuildParams{
		Builder:           blockInfo.Builder,
		Timestamp:         blockInfo.Timestamp,
		L2GasPriceFRI:     blockInfo.L2GasPriceFRI,
		L1GasPriceWEI:     blockInfo.L1GasPriceWEI,
		L1DataGasPriceWEI: blockInfo.L1DataGasPriceWEI,
		EthToStrkRate:     blockInfo.EthToStrkRate,
		L1DAMode:          blockInfo.L1DAMode,
	})
	if err != nil {
		return nil, err
	}

	return &ReceivingTransactionsState{
		Header:     state.Header,
		ValidRound: state.ValidRound,
		BuildState: buildState,
	}, nil
}

func (t *transition) OnTransactions(
	ctx context.Context,
	state *ReceivingTransactionsState,
	transactions []types.Transaction,
) (*ReceivingTransactionsState, error) {
	mempoolTransactions := make([]mempool.BroadcastedTransaction, len(transactions))
	for i := range mempoolTransactions {
		mempoolTransactions[i] = mempool.BroadcastedTransaction{
			Transaction:   transactions[i].Transaction,
			DeclaredClass: transactions[i].Class,
			PaidFeeOnL1:   transactions[i].PaidFeeOnL1,
		}
	}

	if err := t.builder.RunTxns(state.BuildState, mempoolTransactions); err != nil {
		return nil, err
	}

	return state, nil
}

func (t *transition) OnProposalCommitment(
	ctx context.Context,
	state *ReceivingTransactionsState,
	commitment *types.ProposalCommitment,
) (*AwaitingProposalFinState, error) {
	buildResult, err := t.finishAndValidateBlock(state.BuildState, commitment)
	if err != nil {
		return nil, err
	}

	return &AwaitingProposalFinState{
		Proposal: &starknet.Proposal{
			MessageHeader: *state.Header,
			Value:         (*starknet.Value)(buildResult.Preconfirmed.Block.Hash),
			ValidRound:    state.ValidRound,
		},
		BuildResult: &buildResult,
	}, nil
}

func (t *transition) OnProposalFin(
	ctx context.Context,
	state *AwaitingProposalFinState,
	fin *types.ProposalFin,
) (*FinState, error) {
	// Check if expected and actual are both nil or both non-nil
	if (state.Proposal.Value == nil) != (fin == nil) {
		return nil, errHashMismatch
	}

	// If both are non-nil, check if they are equal
	if state.Proposal.Value != nil {
		expected := state.Proposal.Value.Hash()
		actual := starknet.Hash(*fin)
		if expected != actual {
			return nil, errHashMismatch
		}
	}

	return (*FinState)(state), nil
}

func (t *transition) finishAndValidateBlock(
	buildState *builder.BuildState,
	receivedCommitment *types.ProposalCommitment,
) (builder.BuildResult, error) {
	buildResult, err := t.builder.Finish(buildState)
	if err != nil {
		return builder.BuildResult{}, err
	}

	computedCommitment, err := buildResult.ProposalCommitment()
	if err != nil {
		return builder.BuildResult{}, err
	}

	if err := compareProposalCommitment(&computedCommitment, receivedCommitment); err != nil {
		return builder.BuildResult{}, err
	}

	return buildResult, nil
}

// Todo: the validator interface assumes that the msgs are prevalidated before it is called.
// This is an issue because proto3 may leave messages empty, whereas starknet requires them
// to be present
func compareFeltField(name string, received, computed *felt.Felt) error {
	if received.Equal(computed) {
		return nil
	}
	return fmt.Errorf("%s commitment mismatch: received=%s computed=%s", name, received, computed)
}

// Todo: there are fields in ProposalCommitment that we don't check against. Some of these fields
// will be dropped in the finalised spec, so I don't think we should worry about them until then
//  1. Some fields we can't get / compute: VersionConstantCommitment, NextL2GasPriceFRI
//  2. The gas prices. Currently the spec sets eth gas prices, but in v1, these will be dropped
//     for fri prices.
//
//nolint:gocyclo // Currently this is only containing simple equality checks.
func compareProposalCommitment(computed, received *types.ProposalCommitment) error {
	if received.BlockNumber != computed.BlockNumber {
		return fmt.Errorf("block number mismatch: received=%d computed=%d", received.BlockNumber, computed.BlockNumber)
	}

	if !received.ParentCommitment.Equal(&computed.ParentCommitment) {
		return fmt.Errorf("parent hash mismatch: received=%s computed=%s", received.ParentCommitment.String(), computed.ParentCommitment.String())
	}

	if err := compareFeltField("proposer address", &received.Builder, &computed.Builder); err != nil {
		return err
	}

	// Todo: ask the SN guys about the precise checks we should perform with the timestamps
	if received.Timestamp > computed.Timestamp {
		return fmt.Errorf("invalid timestamp: proposal (%d) is later than header (%d)", received.Timestamp, computed.Timestamp)
	}

	if !received.ProtocolVersion.LessThanEqual(builder.CurrentStarknetVersion) {
		return fmt.Errorf("protocol version mismatch: received=%s computed=%s", received.ProtocolVersion, computed.ProtocolVersion)
	}

	// TODO: Validate OldStateRoot, VersionConstantCommitment, NextL2GasPriceFRI

	if err := compareFeltField("state diff", &received.StateDiffCommitment, &computed.StateDiffCommitment); err != nil {
		return err
	}
	if err := compareFeltField("transaction", &received.TransactionCommitment, &computed.TransactionCommitment); err != nil {
		return err
	}
	if err := compareFeltField("event", &received.EventCommitment, &computed.EventCommitment); err != nil {
		return err
	}
	if err := compareFeltField("receipt", &received.ReceiptCommitment, &computed.ReceiptCommitment); err != nil {
		return err
	}
	if err := compareFeltField("concat counts", &received.ConcatenatedCounts, &computed.ConcatenatedCounts); err != nil {
		return err
	}
	if err := compareFeltField("l1 gas price", &received.L1GasPriceFRI, &computed.L1GasPriceFRI); err != nil {
		return err
	}
	if err := compareFeltField("l1 data gas price", &received.L1DataGasPriceFRI, &computed.L1DataGasPriceFRI); err != nil {
		return err
	}
	if err := compareFeltField("l2 gas price", &received.L2GasPriceFRI, &computed.L2GasPriceFRI); err != nil {
		return err
	}
	if err := compareFeltField("l2 gas used", &received.L2GasUsed, &computed.L2GasUsed); err != nil {
		return err
	}

	if received.L1DAMode != computed.L1DAMode {
		return fmt.Errorf("L1 DA mode mismatch: received=%d computed=%d", received.L1DAMode, computed.L1DAMode)
	}

	return nil
}
