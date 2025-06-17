package p2p2consensus

import (
	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	consensus "github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

func AdaptProposalInit(msg *p2pconsensus.ProposalInit) consensus.ProposalInit {
	return consensus.ProposalInit{
		BlockNum: msg.BlockNumber,
		Proposer: *new(felt.Felt).SetBytes(msg.Proposer.Elements),
	}
}

func AdaptBlockInfo(msg *p2pconsensus.BlockInfo) consensus.BlockInfo {
	return consensus.BlockInfo{
		BlockNumber:       msg.BlockNumber,
		Builder:           *new(felt.Felt).SetBytes(msg.Builder.Elements),
		Timestamp:         msg.Timestamp,
		L2GasPriceFRI:     *p2p2core.AdaptUint128(msg.L2GasPriceFri),
		L1GasPriceWEI:     *p2p2core.AdaptUint128(msg.L1DataGasPriceWei),
		L1DataGasPriceWEI: *p2p2core.AdaptUint128(msg.L1DataGasPriceWei),
		EthToStrkRate:     *p2p2core.AdaptUint128(msg.EthToStrkRate),
		L1DAMode:          core.L1DAMode(msg.L1DaMode),
	}
}

func AdaptProposalCommitment(msg *p2pconsensus.ProposalCommitment) consensus.ProposalCommitment {
	return consensus.ProposalCommitment{
		BlockNumber: msg.BlockNumber,
		Builder:     *new(felt.Felt).SetBytes(msg.Builder.Elements),

		ParentCommitment: *new(felt.Felt).SetBytes(msg.ParentCommitment.Elements),
		Timestamp:        msg.Timestamp,
		ProtocolVersion:  *semver.MustParse(msg.ProtocolVersion),

		OldStateRoot:              *new(felt.Felt).SetBytes(msg.OldStateRoot.Elements),
		VersionConstantCommitment: *new(felt.Felt).SetBytes(msg.VersionConstantCommitment.Elements),
		NextL2GasPriceFRI:         *p2p2core.AdaptUint128(msg.NextL2GasPriceFri),

		StateDiffCommitment:   *new(felt.Felt).SetBytes(msg.StateDiffCommitment.Elements),
		TransactionCommitment: *new(felt.Felt).SetBytes(msg.TransactionCommitment.Elements),
		EventCommitment:       *new(felt.Felt).SetBytes(msg.EventCommitment.Elements),
		ReceiptCommitment:     *new(felt.Felt).SetBytes(msg.ReceiptCommitment.Elements),
		ConcatenatedCounts:    *new(felt.Felt).SetBytes(msg.ConcatenatedCounts.Elements),
		L1GasPriceFRI:         *p2p2core.AdaptUint128(msg.L1GasPriceFri),
		L1DataGasPriceFRI:     *p2p2core.AdaptUint128(msg.L1DataGasPriceFri),
		L2GasPriceFRI:         *p2p2core.AdaptUint128(msg.L2GasPriceFri),
		L2GasUsed:             *p2p2core.AdaptUint128(msg.L2GasUsed),
		L1DAMode:              core.L1DAMode(msg.L1DaMode),
	}
}

func AdaptProposalTransaction(msg *p2pconsensus.TransactionBatch, network *utils.Network) []consensus.Transaction {
	txns := make([]consensus.Transaction, len(msg.Transactions))
	for i := range msg.Transactions {
		txn, class := AdaptTransaction(msg.Transactions[i], network)
		txns[i] = consensus.Transaction{
			Transaction: txn,
			Class:       class,
			PaidFeeOnL1: nil, // Todo: this value is not passed in the spec.
		}
	}
	return txns
}

func AdaptProposalFin(msg *p2pconsensus.ProposalFin) consensus.ProposalFin {
	return consensus.ProposalFin(*new(felt.Felt).SetBytes(msg.ProposalCommitment.Elements))
}
