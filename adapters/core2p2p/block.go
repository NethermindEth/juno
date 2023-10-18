package core2p2p

import (
	"github.com/NethermindEth/juno/core"
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

func AdaptEvent(e *core.Event) *spec.Event {
	if e == nil {
		return nil
	}

	return &spec.Event{
		FromAddress: AdaptFelt(e.From),
		Keys:        utils.Map(e.Keys, AdaptFelt),
		Data:        utils.Map(e.Data, AdaptFelt),
	}
}
