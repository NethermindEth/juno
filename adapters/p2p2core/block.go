package p2p2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bloom/v3"
)

func AdaptEvent(e *spec.Event) *core.Event {
	if e == nil {
		return nil
	}

	return &core.Event{
		From: AdaptFelt(e.FromAddress),
		Keys: utils.Map(e.Keys, AdaptFelt),
		Data: utils.Map(e.Data, AdaptFelt),
	}
}

func AdaptBlockHeader(h *spec.SignedBlockHeader, eventsBloom *bloom.BloomFilter) *core.Header {
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
		Signatures:       utils.Map(h.Signatures, adaptSignature),
		L1DAMode:         adaptDA(h.L1DataAvailabilityMode),
		L1DataGasPrice: &core.GasPrice{
			PriceInWei: AdaptUint128(h.DataGasPriceWei),
			PriceInFri: AdaptUint128(h.DataGasPriceFri),
		},
		GasPrice:     AdaptUint128(h.GasPriceFri),
		GasPriceSTRK: AdaptUint128(h.GasPriceWei),
	}
}

func adaptSignature(cs *spec.ConsensusSignature) []*felt.Felt {
	return []*felt.Felt{AdaptFelt(cs.R), AdaptFelt(cs.S)}
}

func adaptDA(da spec.L1DataAvailabilityMode) core.L1DAMode {
	switch da {
	case spec.L1DataAvailabilityMode_Calldata:
		return core.Calldata
	case spec.L1DataAvailabilityMode_Blob:
		return core.Blob
	default:
		panic(fmt.Errorf("unsupported DA mode %v", da))
	}
}
