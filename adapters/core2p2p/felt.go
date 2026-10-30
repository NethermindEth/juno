package core2p2p

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/transaction"
)

func AdaptHash[K any](f *felt.Typed[K]) *common.Hash {
	if f == nil {
		return nil
	}

	return &common.Hash{
		Elements: f.Marshal(),
	}
}

func AdaptAccountSignature(signature []felt.Felt) *transaction.AccountSignature {
	return &transaction.AccountSignature{
		Parts: AdaptFeltSlice(signature),
	}
}

func AdaptFelt[K any](f *felt.Typed[K]) *common.Felt252 {
	if f == nil {
		return nil
	}

	return &common.Felt252{
		Elements: f.Marshal(),
	}
}

func AdaptFeltSlice(slice []felt.Felt) []*common.Felt252 {
	if slice == nil {
		return nil
	}
	result := make([]*common.Felt252, len(slice))
	for idx := range slice {
		result[idx] = AdaptFelt(&slice[idx])
	}
	return result
}

func AdaptAddress[K any](f *felt.Typed[K]) *common.Address {
	if f == nil {
		return nil
	}

	return &common.Address{
		Elements: f.Marshal(),
	}
}

func AdaptUint128[K any](f *felt.Typed[K]) *common.Uint128 {
	// bits represents value in little endian byte order
	// i.e. first is least significant byte
	bits := f.Bits()
	return &common.Uint128{
		Low:  bits[0],
		High: bits[1],
	}
}
