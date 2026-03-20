package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	synccommon "github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/common"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/event"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/header"
)

func AdaptBlockID(blockHeader *core.Header) *common.BlockID {
	if blockHeader == nil {
		return nil
	}

	return &common.BlockID{
		Number: blockHeader.Number,
		Header: AdaptHash(blockHeader.Hash),
	}
}

func adaptSignature(sig []*felt.Felt) *common.ConsensusSignature {
	return &common.ConsensusSignature{
		R: AdaptFelt(sig[0]),
		S: AdaptFelt(sig[1]),
	}
}

func AdaptHeader(blockHeader *core.Header, commitments *core.BlockCommitments,
	stateDiffCommitment *felt.Felt, stateDiffLength uint64,
) *header.SignedBlockHeader {
	var l1DataGasPriceFri, l1DataGasPriceWei, l2GasPriceFri, l2GasPriceWei *felt.Felt
	if l1DataGasPrice := blockHeader.L1DataGasPrice; l1DataGasPrice != nil {
		l1DataGasPriceFri = l1DataGasPrice.PriceInFri
		l1DataGasPriceWei = l1DataGasPrice.PriceInWei
	} else {
		l1DataGasPriceFri = &felt.Zero
		l1DataGasPriceWei = &felt.Zero
	}
	if l2GasPrice := blockHeader.L2GasPrice; l2GasPrice != nil {
		l2GasPriceFri = l2GasPrice.PriceInFri
		l2GasPriceWei = l2GasPrice.PriceInWei
	} else {
		l2GasPriceFri = &felt.Zero
		l2GasPriceWei = &felt.Zero
	}
	return &header.SignedBlockHeader{
		BlockHash:        AdaptHash(blockHeader.Hash),
		ParentHash:       AdaptHash(blockHeader.ParentHash),
		Number:           blockHeader.Number,
		Time:             blockHeader.Timestamp,
		SequencerAddress: AdaptAddress(blockHeader.SequencerAddress),
		StateRoot:        AdaptHash(blockHeader.GlobalStateRoot),
		Transactions: &common.Patricia{
			NLeaves: blockHeader.TransactionCount,
			Root:    AdaptHash(commitments.TransactionCommitment),
		},
		Events: &common.Patricia{
			NLeaves: blockHeader.EventCount,
			Root:    AdaptHash(commitments.EventCommitment),
		},
		Receipts:        AdaptHash(commitments.ReceiptCommitment),
		ProtocolVersion: blockHeader.ProtocolVersion,
		L1GasPriceFri:   AdaptUint128(blockHeader.L1GasPriceSTRK),
		Signatures:      utils.Map(blockHeader.Signatures, adaptSignature),
		StateDiffCommitment: &synccommon.StateDiffCommitment{
			StateDiffLength: stateDiffLength,
			Root:            AdaptHash(stateDiffCommitment),
		},
		L1GasPriceWei:          AdaptUint128(blockHeader.L1GasPriceETH),
		L1DataGasPriceFri:      AdaptUint128(l1DataGasPriceFri),
		L1DataGasPriceWei:      AdaptUint128(l1DataGasPriceWei),
		L1DataAvailabilityMode: adaptL1DA(blockHeader.L1DAMode),
		L2GasPriceFri:          AdaptUint128(l2GasPriceFri),
		L2GasPriceWei:          AdaptUint128(l2GasPriceWei),
	}
}

func adaptL1DA(da core.L1DAMode) common.L1DataAvailabilityMode {
	switch da {
	case core.Calldata:
		return common.L1DataAvailabilityMode_Calldata
	case core.Blob:
		return common.L1DataAvailabilityMode_Blob
	default:
		panic(fmt.Errorf("unknown L1DAMode %v", da))
	}
}

func AdaptEvent(e *core.Event, txH *felt.Felt) *event.Event {
	if e == nil {
		return nil
	}

	return &event.Event{
		TransactionHash: AdaptHash(txH),
		FromAddress:     AdaptFelt(e.From),
		Keys:            utils.Map(e.Keys, AdaptFelt),
		Data:            utils.Map(e.Data, AdaptFelt),
	}
}
