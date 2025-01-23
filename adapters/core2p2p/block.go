package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/gen"
	"github.com/NethermindEth/juno/utils"
)

func AdaptBlockID(header *core.Header) *gen.BlockID {
	if header == nil {
		return nil
	}

	return &gen.BlockID{
		Number: header.Number,
		Header: AdaptHash(header.Hash),
	}
}

func adaptSignature(sig []*felt.Felt) *gen.ConsensusSignature {
	return &gen.ConsensusSignature{
		R: AdaptFelt(sig[0]),
		S: AdaptFelt(sig[1]),
	}
}

func AdaptHeader(header *core.Header, commitments *core.BlockCommitments,
	stateDiffCommitment *felt.Felt, stateDiffLength uint64,
) *gen.SignedBlockHeader {
	var l1DataGasPriceFri, l1DataGasPriceWei, l2GasPriceFri, l2GasPriceWei *felt.Felt
	if l1DataGasPrice := header.L1DataGasPrice; l1DataGasPrice != nil {
		l1DataGasPriceFri = l1DataGasPrice.PriceInFri
		l1DataGasPriceWei = l1DataGasPrice.PriceInWei
	} else {
		l1DataGasPriceFri = &felt.Zero
		l1DataGasPriceWei = &felt.Zero
	}
	if l2GasPrice := header.L2GasPrice; l2GasPrice != nil {
		l2GasPriceFri = l2GasPrice.PriceInFri
		l2GasPriceWei = l2GasPrice.PriceInWei
	} else {
		l2GasPriceFri = &felt.Zero
		l2GasPriceWei = &felt.Zero
	}
	return &gen.SignedBlockHeader{
		BlockHash:        AdaptHash(header.Hash),
		ParentHash:       AdaptHash(header.ParentHash),
		Number:           header.Number,
		Time:             header.Timestamp,
		SequencerAddress: AdaptAddress(header.SequencerAddress),
		StateRoot:        AdaptHash(header.GlobalStateRoot),
		Transactions: &gen.Patricia{
			NLeaves: header.TransactionCount,
			Root:    AdaptHash(commitments.TransactionCommitment),
		},
		Events: &gen.Patricia{
			NLeaves: header.EventCount,
			Root:    AdaptHash(commitments.EventCommitment),
		},
		Receipts:        AdaptHash(commitments.ReceiptCommitment),
		ProtocolVersion: header.ProtocolVersion,
		L1GasPriceFri:   AdaptUint128(header.L1GasPriceSTRK),
		Signatures:      utils.Map(header.Signatures, adaptSignature),
		StateDiffCommitment: &gen.StateDiffCommitment{
			StateDiffLength: stateDiffLength,
			Root:            AdaptHash(stateDiffCommitment),
		},
		L1GasPriceWei:          AdaptUint128(header.L1GasPriceETH),
		L1DataGasPriceFri:      AdaptUint128(l1DataGasPriceFri),
		L1DataGasPriceWei:      AdaptUint128(l1DataGasPriceWei),
		L1DataAvailabilityMode: adaptL1DA(header.L1DAMode),
		L2GasPriceFri:          AdaptUint128(l2GasPriceFri),
		L2GasPriceWei:          AdaptUint128(l2GasPriceWei),
	}
}

func adaptL1DA(da core.L1DAMode) gen.L1DataAvailabilityMode {
	switch da {
	case core.Calldata:
		return gen.L1DataAvailabilityMode_Calldata
	case core.Blob:
		return gen.L1DataAvailabilityMode_Blob
	default:
		panic(fmt.Errorf("unknown L1DAMode %v", da))
	}
}

func AdaptEvent(e *core.Event, txH *felt.Felt) *gen.Event {
	if e == nil {
		return nil
	}

	return &gen.Event{
		TransactionHash: AdaptHash(txH),
		FromAddress:     AdaptFelt(e.From),
		Keys:            utils.Map(e.Keys, AdaptFelt),
		Data:            utils.Map(e.Data, AdaptFelt),
	}
}
