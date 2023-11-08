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
