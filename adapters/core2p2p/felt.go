package core2p2p

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/gen"
	"github.com/NethermindEth/juno/utils"
)

func AdaptHash(f *felt.Felt) *gen.Hash {
	if f == nil {
		return nil
	}

	return &gen.Hash{
		Elements: f.Marshal(),
	}
}

func AdaptAccountSignature(signature []*felt.Felt) *gen.AccountSignature {
	return &gen.AccountSignature{
		Parts: utils.Map(signature, AdaptFelt),
	}
}

func AdaptFelt(f *felt.Felt) *gen.Felt252 {
	if f == nil {
		return nil
	}

	return &gen.Felt252{
		Elements: f.Marshal(),
	}
}

func AdaptFeltSlice(sl []*felt.Felt) []*gen.Felt252 {
	return utils.Map(sl, AdaptFelt)
}

func AdaptAddress(f *felt.Felt) *gen.Address {
	if f == nil {
		return nil
	}

	return &gen.Address{
		Elements: f.Marshal(),
	}
}

func AdaptUint128(f *felt.Felt) *gen.Uint128 {
	// bits represents value in little endian byte order
	// i.e. first is least significant byte
	bits := f.Bits()
	return &gen.Uint128{
		Low:  bits[0],
		High: bits[1],
	}
}
