package p2p2consensus

import (
	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	consensus "github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

func AdaptProposalInit(msg *p2pconsensus.ProposalInit) (consensus.ProposalInit, error) {
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
	return consensus.BlockInfo{
		BlockNumber:       msg.BlockNumber,
		Builder:           *new(felt.Felt).SetBytes(msg.Builder.Elements),
		Timestamp:         msg.Timestamp,
		L2GasPriceFRI:     *p2p2core.AdaptUint128(msg.L2GasPriceFri),
		L1GasPriceWEI:     *p2p2core.AdaptUint128(msg.L1GasPriceWei),
		L1DataGasPriceWEI: *p2p2core.AdaptUint128(msg.L1DataGasPriceWei),
		EthToStrkRate:     *p2p2core.AdaptUint128(msg.EthToStrkRate),
		L1DAMode:          core.L1DAMode(msg.L1DaMode),
	}, nil
}

func AdaptProposalCommitment(msg *p2pconsensus.ProposalCommitment) (consensus.ProposalCommitment, error) {
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
	}, nil
}

func AdaptProposalTransaction(msg *p2pconsensus.TransactionBatch) ([]consensus.Transaction, error) {
	var err error
	txns := make([]consensus.Transaction, len(msg.Transactions))
	for i := range msg.Transactions {
		if txns[i], err = AdaptTransaction(msg.Transactions[i]); err != nil {
			return nil, err
		}
	}
	return txns, nil
}

func AdaptProposalFin(msg *p2pconsensus.ProposalFin) (consensus.ProposalFin, error) {
	return consensus.ProposalFin(*new(felt.Felt).SetBytes(msg.ProposalCommitment.Elements)), nil
}
