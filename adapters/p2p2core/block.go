package p2p2core

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
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

func AdaptSignature(cs *spec.ConsensusSignature) []*felt.Felt {
	return []*felt.Felt{AdaptFelt(cs.R), AdaptFelt(cs.S)}
}

func AdaptBlockHeader(h *spec.BlockHeader) core.Header {
	return core.Header{
		Hash:             nil, // todo: add this when building the block
		ParentHash:       AdaptHash(h.ParentHash),
		Number:           h.Number,
		GlobalStateRoot:  AdaptHash(h.State.Root),
		SequencerAddress: AdaptAddress(h.SequencerAddress),
		TransactionCount: uint64(h.Transactions.NLeaves),
		EventCount:       uint64(h.Events.NLeaves),
		Timestamp:        uint64(h.Time.AsTime().Unix()),
		ProtocolVersion:  h.ProtocolVersion,
		EventsBloom:      nil, // Todo: add this in when building the block
		GasPrice:         AdaptFelt(h.GasPrice),
	}
}
