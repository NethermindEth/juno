package starknet

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

func AdaptFeltToHash(f *felt.Felt) *spec.Hash {
	fBytes := f.Bytes()
	return &spec.Hash{
		Elements: fBytes[:],
	}
}

func adaptFeltToAddress(f *felt.Felt) *spec.Address {
	fBytes := f.Bytes()
	return &spec.Address{
		Elements: fBytes[:],
	}
}

func AdaptFelt(f *felt.Felt) *spec.Felt252 {
	fBytes := f.Bytes()
	return &spec.Felt252{
		Elements: fBytes[:],
	}
}
