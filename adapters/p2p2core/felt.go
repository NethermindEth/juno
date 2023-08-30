package p2p2core

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

func AdaptHash(h *spec.Hash) *felt.Felt {
	if h == nil {
		return nil
	}

	return new(felt.Felt).SetBytes(h.Elements)
}
