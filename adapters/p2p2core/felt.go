package p2p2core

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/ethereum/go-ethereum/common"
)

func AdaptHash(h *spec.Hash) *felt.Felt {
	return Adapt(h)
}

func AdaptAddress(h *spec.Address) *felt.Felt {
	return Adapt(h)
}

func AdaptEthAddress(h *spec.EthereumAddress) common.Address {
	return common.BytesToAddress(h.Elements)
}

func AdaptFelt(f *spec.Felt252) *felt.Felt {
	return Adapt(f)
}

type feltElements interface {
	GetElements() []byte
}

func Adapt(f feltElements) *felt.Felt {
	if f == nil {
		return nil
	}

	return new(felt.Felt).SetBytes(f.GetElements())
}
