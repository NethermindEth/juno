package p2p2consensus

import (
	"context"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	consensus "github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

func AdaptProposalInit(msg *p2pconsensus.ProposalInit) (consensus.ProposalInit, error) {
	if err := validateProposalInit(msg); err != nil {
		return consensus.ProposalInit{}, err
	}

	var proposer felt.Felt
	if err := proposer.SetBytesCanonical(msg.Proposer.Elements); err != nil {
		return consensus.ProposalInit{}, err
	}

	var validRound consensus.Round = -1
	if msg.ValidRound != nil {
		validRound = consensus.Round(*msg.ValidRound)
	}

	return consensus.ProposalInit{
		BlockNum:   consensus.Height(msg.BlockNumber),
		Round:      consensus.Round(msg.Round),
		ValidRound: validRound,
		Proposer:   *new(felt.Felt).SetBytes(msg.Proposer.Elements),
	}, nil
}

func AdaptBlockInfo(msg *p2pconsensus.BlockInfo) (consensus.BlockInfo, error) {
	if err := validateBlockInfo(msg); err != nil {
		return consensus.BlockInfo{}, err
	}

	var builder felt.Felt
	if err := builder.SetBytesCanonical(msg.Builder.Elements); err != nil {
		return consensus.BlockInfo{}, err
	}

	return consensus.BlockInfo{
		BlockNumber:       msg.BlockNumber,
		Builder:           builder,
		Timestamp:         msg.Timestamp,
		L2GasPriceFRI:     *p2p2core.AdaptUint128(msg.L2GasPriceFri),
		L1GasPriceWEI:     *p2p2core.AdaptUint128(msg.L1GasPriceWei),
		L1DataGasPriceWEI: *p2p2core.AdaptUint128(msg.L1DataGasPriceWei),
		EthToStrkRate:     *p2p2core.AdaptUint128(msg.EthToStrkRate),
		L1DAMode:          core.L1DAMode(msg.L1DaMode),
	}, nil
}

func AdaptProposalTransaction(
	ctx context.Context,
	compiler compiler.Compiler,
	msg *p2pconsensus.TransactionBatch,
	network *utils.Network,
) ([]consensus.Transaction, error) {
	var err error
	txns := make([]consensus.Transaction, len(msg.Transactions))
	for i := range msg.Transactions {
		if txns[i], err = AdaptTransaction(ctx, compiler, msg.Transactions[i], network); err != nil {
			return nil, err
		}
	}
	return txns, nil
}

func AdaptProposalCommitment(msg *p2pconsensus.ProposalCommitment) (consensus.ProposalCommitment, error) {
	if err := validateProposalCommitment(msg); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	var builder felt.Felt
	if err := builder.SetBytesCanonical(msg.Builder.Elements); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	var parentCommitment felt.Felt
	if err := parentCommitment.SetBytesCanonical(msg.ParentCommitment.Elements); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	var oldStateRoot felt.Felt
	if err := oldStateRoot.SetBytesCanonical(msg.OldStateRoot.Elements); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	var versionConstantCommitment felt.Felt
	if err := versionConstantCommitment.SetBytesCanonical(msg.VersionConstantCommitment.Elements); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	var stateDiffCommitment felt.Felt
	if err := stateDiffCommitment.SetBytesCanonical(msg.StateDiffCommitment.Elements); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	var transactionCommitment felt.Felt
	if err := transactionCommitment.SetBytesCanonical(msg.TransactionCommitment.Elements); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	var eventCommitment felt.Felt
	if err := eventCommitment.SetBytesCanonical(msg.EventCommitment.Elements); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	var receiptCommitment felt.Felt
	if err := receiptCommitment.SetBytesCanonical(msg.ReceiptCommitment.Elements); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	var concatenatedCounts felt.Felt
	if err := concatenatedCounts.SetBytesCanonical(msg.ConcatenatedCounts.Elements); err != nil {
		return consensus.ProposalCommitment{}, err
	}

	snVersion, err := semver.NewVersion(msg.ProtocolVersion)
	if err != nil {
		return consensus.ProposalCommitment{}, err
	}

	return consensus.ProposalCommitment{
		BlockNumber:               msg.BlockNumber,
		Builder:                   builder,
		ParentCommitment:          parentCommitment,
		Timestamp:                 msg.Timestamp,
		ProtocolVersion:           *snVersion,
		OldStateRoot:              oldStateRoot,
		VersionConstantCommitment: versionConstantCommitment,
		NextL2GasPriceFRI:         *p2p2core.AdaptUint128(msg.NextL2GasPriceFri),
		StateDiffCommitment:       stateDiffCommitment,
		TransactionCommitment:     transactionCommitment,
		EventCommitment:           eventCommitment,
		ReceiptCommitment:         receiptCommitment,
		ConcatenatedCounts:        concatenatedCounts,
		L1GasPriceFRI:             *p2p2core.AdaptUint128(msg.L1GasPriceFri),
		L1DataGasPriceFRI:         *p2p2core.AdaptUint128(msg.L1DataGasPriceFri),
		L2GasPriceFRI:             *p2p2core.AdaptUint128(msg.L2GasPriceFri),
		L2GasUsed:                 *p2p2core.AdaptUint128(msg.L2GasUsed),
		L1DAMode:                  core.L1DAMode(msg.L1DaMode),
	}, nil
}

func AdaptProposalFin(msg *p2pconsensus.ProposalFin) (consensus.ProposalFin, error) {
	if err := validateProposalFin(msg); err != nil {
		return consensus.ProposalFin{}, err
	}

	var proposalCommitment felt.Felt
	if err := proposalCommitment.SetBytesCanonical(msg.ProposalCommitment.Elements); err != nil {
		return consensus.ProposalFin{}, err
	}

	return consensus.ProposalFin(proposalCommitment), nil
}
