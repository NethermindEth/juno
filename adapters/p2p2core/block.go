package p2p2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/event"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/header"
)

func AdaptEvent(e *event.Event) *core.Event {
	if e == nil {
		return nil
	}

	return &core.Event{
		From: AdaptFelt[felt.Felt](e.FromAddress),
		Keys: AdaptFeltSlice(e.Keys),
		Data: AdaptFeltSlice(e.Data),
	}
}

func AdaptBlockHeader(
	h *header.SignedBlockHeader,
) (*core.Header, error) {
	l1DataGasPrice, err := adaptDA(h.L1DataAvailabilityMode)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch L1DAMode: %w", err)
	}

	return &core.Header{
		Hash:             AdaptHash[felt.Felt](h.BlockHash),
		ParentHash:       AdaptHash[felt.Felt](h.ParentHash),
		Number:           h.Number,
		GlobalStateRoot:  AdaptHash[felt.Felt](h.StateRoot),
		SequencerAddress: AdaptAddress(h.SequencerAddress),
		TransactionCount: h.Transactions.NLeaves,
		EventCount:       h.Events.NLeaves,
		Timestamp:        h.Time,
		ProtocolVersion:  h.ProtocolVersion,
		L1GasPriceETH:    AdaptUint128[felt.Felt](h.L1GasPriceWei),
		Signatures:       utils.Map(h.Signatures, adaptSignature),
		L1GasPriceSTRK:   AdaptUint128[felt.Felt](h.L1GasPriceFri),
		L1DAMode:         l1DataGasPrice,
		L1DataGasPrice: &core.GasPrice{
			PriceInWei: AdaptUint128[felt.Felt](h.L1DataGasPriceWei),
			PriceInFri: AdaptUint128[felt.Felt](h.L1DataGasPriceFri),
		},
		L2GasPrice: &core.GasPrice{
			PriceInWei: AdaptUint128[felt.Felt](h.L2GasPriceWei),
			PriceInFri: AdaptUint128[felt.Felt](h.L2GasPriceFri),
		},
	}, nil
}

func adaptSignature(cs *common.ConsensusSignature) []*felt.Felt {
	return []*felt.Felt{AdaptFelt[felt.Felt](cs.R), AdaptFelt[felt.Felt](cs.S)}
}

func adaptDA(da common.L1DataAvailabilityMode) (core.L1DAMode, error) {
	switch da {
	case common.L1DataAvailabilityMode_Calldata:
		return core.Calldata, nil
	case common.L1DataAvailabilityMode_Blob:
		return core.Blob, nil
	default:
		return 0, fmt.Errorf("unsupported DA mode: %v", da)
	}
}
