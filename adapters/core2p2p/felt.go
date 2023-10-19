package core2p2p

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

func AdaptHash(f *felt.Felt) *spec.Hash {
	if f == nil {
		return nil
	}

	return &spec.Hash{
		Elements: f.Marshal(),
	}
}

func AdaptFelt(f *felt.Felt) *spec.Felt252 {
	if f == nil {
		return nil
	}

	return &spec.Felt252{
		Elements: f.Marshal(),
	}
}

func AdaptAddress(f *felt.Felt) *spec.Address {
	if f == nil {
		return nil
	}

	return &spec.Address{
		Elements: f.Marshal(),
	}
}
