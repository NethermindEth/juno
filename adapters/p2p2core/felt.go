package p2p2core

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/receipt"
)

func AdaptHash[F felt.FeltLike](h *common.Hash) *F {
	return adapt[F](h)
}

func AdaptAddress(h *common.Address) *felt.Felt {
	return adapt[felt.Felt](h)
}

func AdaptEthAddress(h *receipt.EthereumAddress) eth.Address {
	return eth.AddressFromBytes(h.Elements)
}

func AdaptFelt[F felt.FeltLike](f *common.Felt252) *F {
	return adapt[F](f)
}

func AdaptFeltSlice(slice []*common.Felt252) []felt.Felt {
	if slice == nil {
		return nil
	}
	result := make([]felt.Felt, len(slice))
	for idx := range slice {
		result[idx] = felt.FromBytes[felt.Felt](slice[idx].GetElements())
	}
	return result
}

func adapt[F felt.FeltLike](v interface{ GetElements() []byte }) *F {
	if utils.IsNil(v) {
		return nil
	}

	return felt.NewFromBytes[F](v.GetElements())
}

func AdaptUint128[F felt.FeltLike](u *common.Uint128) *F {
	if u == nil {
		return nil
	}

	bytes := make([]byte, 16)

	binary.BigEndian.PutUint64(bytes[:8], u.High)
	binary.BigEndian.PutUint64(bytes[8:], u.Low)

	return felt.NewFromBytes[F](bytes)
}
