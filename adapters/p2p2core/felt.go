package p2p2core

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/ethereum/go-ethereum/common"
)

func AdaptHash(h *spec.Hash) *felt.Felt {
	return adapt(h)
}

func AdaptAddress(h *spec.Address) *felt.Felt {
	return adapt(h)
}

func AdaptEthAddress(h *spec.EthereumAddress) common.Address {
	return common.BytesToAddress(h.Elements)
}

func AdaptFelt(f *spec.Felt252) *felt.Felt {
	return adapt(f)
}

func adapt(v interface{ GetElements() []byte }) *felt.Felt {
	if v == nil {
		return nil
	}

	return new(felt.Felt).SetBytes(v.GetElements())
}
