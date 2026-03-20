package consensus2p2p

import (
	"github.com/NethermindEth/juno/adapters/core2p2p"
	consensus "github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	p2pconsensus "github.com/starknet-io/starknet-p2p-specs/p2p/proto/consensus/consensus"
)

func toAddress(val *felt.Felt) *common.Address {
	feltBytes := val.Bytes()
	return &common.Address{Elements: feltBytes[:]}
}

func toHash(val *felt.Felt) *common.Hash {
	feltBytes := val.Bytes()
	return &common.Hash{Elements: feltBytes[:]}
}

func toFelt252(val *felt.Felt) *common.Felt252 {
	feltBytes := val.Bytes()
	return &common.Felt252{Elements: feltBytes[:]}
}

func AdaptProposalInit(msg *consensus.ProposalInit) p2pconsensus.ProposalInit {
	var validRound *uint32
	if msg.ValidRound >= 0 {
		validRound = utils.HeapPtr(uint32(msg.ValidRound))
	}

	return p2pconsensus.ProposalInit{
		BlockNumber: uint64(msg.BlockNum),
		Round:       uint32(msg.Round),
		ValidRound:  validRound,
		Proposer:    toAddress(&msg.Proposer),
	}
}

func AdaptBlockInfo(msg *consensus.BlockInfo) p2pconsensus.BlockInfo {
	return p2pconsensus.BlockInfo{
		BlockNumber:       msg.BlockNumber,
		Builder:           toAddress(&msg.Builder),
		Timestamp:         msg.Timestamp,
		L2GasPriceFri:     core2p2p.AdaptUint128(&msg.L2GasPriceFRI),
		L1GasPriceWei:     core2p2p.AdaptUint128(&msg.L1GasPriceWEI),
		L1DataGasPriceWei: core2p2p.AdaptUint128(&msg.L1DataGasPriceWEI),
		EthToStrkRate:     core2p2p.AdaptUint128(&msg.EthToStrkRate),
		L1DaMode:          common.L1DataAvailabilityMode(msg.L1DAMode),
	}
}

func AdaptProposalCommitment(msg *consensus.ProposalCommitment) p2pconsensus.ProposalCommitment {
	return p2pconsensus.ProposalCommitment{
		BlockNumber:               msg.BlockNumber,
		ParentCommitment:          toHash(&msg.ParentCommitment),
		Builder:                   toAddress(&msg.Builder),
		Timestamp:                 msg.Timestamp,
		ProtocolVersion:           msg.ProtocolVersion.String(),
		OldStateRoot:              toHash(&msg.OldStateRoot),
		VersionConstantCommitment: toHash(&msg.VersionConstantCommitment),
		StateDiffCommitment:       toHash(&msg.StateDiffCommitment),
		TransactionCommitment:     toHash(&msg.TransactionCommitment),
		EventCommitment:           toHash(&msg.EventCommitment),
		ReceiptCommitment:         toHash(&msg.ReceiptCommitment),
		ConcatenatedCounts:        toFelt252(&msg.ConcatenatedCounts),
		L1GasPriceFri:             core2p2p.AdaptUint128(&msg.L1GasPriceFRI),
		L1DataGasPriceFri:         core2p2p.AdaptUint128(&msg.L1DataGasPriceFRI),
		L2GasPriceFri:             core2p2p.AdaptUint128(&msg.L2GasPriceFRI),
		L2GasUsed:                 core2p2p.AdaptUint128(&msg.L2GasUsed),
		NextL2GasPriceFri:         core2p2p.AdaptUint128(&msg.NextL2GasPriceFRI),
		L1DaMode:                  common.L1DataAvailabilityMode(msg.L1DAMode),
	}
}

func AdaptProposalTransaction(msg []consensus.Transaction) (p2pconsensus.TransactionBatch, error) {
	var err error
	txns := make([]*p2pconsensus.ConsensusTransaction, len(msg))
	for i := range msg {
		if txns[i], err = AdaptTransaction(&msg[i]); err != nil {
			return p2pconsensus.TransactionBatch{}, err
		}
	}
	return p2pconsensus.TransactionBatch{
		Transactions: txns,
	}, nil
}

func AdaptProposalFin(msg *consensus.ProposalFin) p2pconsensus.ProposalFin {
	return p2pconsensus.ProposalFin{
		ProposalCommitment: toHash((*felt.Felt)(msg)),
	}
}
