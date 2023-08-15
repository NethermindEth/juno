package core2p2p

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

func AdaptHash(f *felt.Felt) *spec.Hash {
	return &spec.Hash{
		Elements: f.Marshal(),
	}
}

func AdaptAddress(f *felt.Felt) *spec.Address {
	return &spec.Address{
		Elements: f.Marshal(),
	}
}

func AdaptChainID(f *felt.Felt) *spec.ChainID {
	return &spec.ChainID{
		Id: f.Marshal(),
	}
}
