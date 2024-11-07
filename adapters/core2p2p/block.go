package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptBlockID(header *core.Header) *spec.BlockID {
	if header == nil {
		return nil
	}

	return &spec.BlockID{
		Number: header.Number,
		Header: AdaptHash(header.Hash),
	}
}

func adaptSignature(sig []*felt.Felt) *spec.ConsensusSignature {
	return &spec.ConsensusSignature{
		R: AdaptFelt(sig[0]),
		S: AdaptFelt(sig[1]),
	}
}

func AdaptHeader(header *core.Header, commitments *core.BlockCommitments,
	stateDiffCommitment *felt.Felt, stateDiffLength uint64,
) *spec.SignedBlockHeader {
	return &spec.SignedBlockHeader{
		BlockHash:        AdaptHash(header.Hash),
		ParentHash:       AdaptHash(header.ParentHash),
		Number:           header.Number,
		Time:             header.Timestamp,
		SequencerAddress: AdaptAddress(header.SequencerAddress),
		StateRoot:        AdaptHash(header.GlobalStateRoot),
		Transactions: &spec.Patricia{
			NLeaves: header.TransactionCount,
			Root:    AdaptHash(commitments.TransactionCommitment),
		},
		Events: &spec.Patricia{
			NLeaves: header.EventCount,
			Root:    AdaptHash(commitments.EventCommitment),
		},
		Receipts:        AdaptHash(commitments.ReceiptCommitment),
		ProtocolVersion: header.ProtocolVersion,
		GasPriceFri:     AdaptUint128(header.GasPriceSTRK),
		Signatures:      utils.Map(header.Signatures, adaptSignature),
		StateDiffCommitment: &spec.StateDiffCommitment{
			StateDiffLength: stateDiffLength,
			Root:            AdaptHash(stateDiffCommitment),
		},
		GasPriceWei:            AdaptUint128(header.GasPrice),
		DataGasPriceFri:        AdaptUint128(header.L1DataGasPrice.PriceInFri),
		DataGasPriceWei:        AdaptUint128(header.L1DataGasPrice.PriceInWei),
		L1DataAvailabilityMode: adaptL1DA(header.L1DAMode),
	}
}

func adaptL1DA(da core.L1DAMode) spec.L1DataAvailabilityMode {
	switch da {
	case core.Calldata:
		return spec.L1DataAvailabilityMode_Calldata
	case core.Blob:
		return spec.L1DataAvailabilityMode_Blob
	default:
		panic(fmt.Errorf("unknown L1DAMode %v", da))
	}
}

func AdaptEvent(e *core.Event, txH *felt.Felt) *spec.Event {
	if e == nil {
		return nil
	}

	return &spec.Event{
		TransactionHash: AdaptHash(txH),
		FromAddress:     AdaptFelt(e.From),
		Keys:            utils.Map(e.Keys, AdaptFelt),
		Data:            utils.Map(e.Data, AdaptFelt),
	}
}
