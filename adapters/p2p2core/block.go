package p2p2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/event"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/header"
)

func AdaptEvent(e *event.Event) *core.Event {
	if e == nil {
		return nil
	}

	return &core.Event{
		From: AdaptFelt(e.FromAddress),
		Keys: utils.Map(e.Keys, AdaptFelt),
		Data: utils.Map(e.Data, AdaptFelt),
	}
}

func AdaptBlockHeader(
	h *header.SignedBlockHeader,
	eventsBloom *bloom.BloomFilter,
) (*core.Header, error) {
	l1DataGasPrice, err := adaptDA(h.L1DataAvailabilityMode)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch L1DAMode: %w", err)
	}

	return &core.Header{
		Hash:             AdaptHash(h.BlockHash),
		ParentHash:       AdaptHash(h.ParentHash),
		Number:           h.Number,
		GlobalStateRoot:  AdaptHash(h.StateRoot),
		SequencerAddress: AdaptAddress(h.SequencerAddress),
		TransactionCount: h.Transactions.NLeaves,
		EventCount:       h.Events.NLeaves,
		Timestamp:        h.Time,
		ProtocolVersion:  h.ProtocolVersion,
		EventsBloom:      eventsBloom,
		L1GasPriceETH:    AdaptUint128(h.L1GasPriceWei),
		Signatures:       utils.Map(h.Signatures, adaptSignature),
		L1GasPriceSTRK:   AdaptUint128(h.L1GasPriceFri),
		L1DAMode:         l1DataGasPrice,
		L1DataGasPrice: &core.GasPrice{
			PriceInWei: AdaptUint128(h.L1DataGasPriceWei),
			PriceInFri: AdaptUint128(h.L1DataGasPriceFri),
		},
		L2GasPrice: &core.GasPrice{
			PriceInWei: AdaptUint128(h.L2GasPriceWei),
			PriceInFri: AdaptUint128(h.L2GasPriceFri),
		},
	}, nil
}

func adaptSignature(cs *common.ConsensusSignature) []*felt.Felt {
	return []*felt.Felt{AdaptFelt(cs.R), AdaptFelt(cs.S)}
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
