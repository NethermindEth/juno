package p2p2core

import (
	"reflect"

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
	// when passing a nil pointer `v` is boxed to an interface and is not nil
	// see: https://blog.devtrovert.com/p/go-secret-interface-nil-is-not-nil
	if v == nil || reflect.ValueOf(v).IsNil() {
		return nil
	}

	return new(felt.Felt).SetBytes(v.GetElements())
}

func AdaptUint128(u *spec.Uint128) *felt.Felt {
	// todo handle u128
	return &felt.Zero
}
