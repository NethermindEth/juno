package p2p2core

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/gen"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

func AdaptHash(h *gen.Hash) *felt.Felt {
	return adapt(h)
}

func AdaptAddress(h *gen.Address) *felt.Felt {
	return adapt(h)
}

func AdaptEthAddress(h *gen.EthereumAddress) common.Address {
	return common.BytesToAddress(h.Elements)
}

func AdaptFelt(f *gen.Felt252) *felt.Felt {
	return adapt(f)
}

func adapt(v interface{ GetElements() []byte }) *felt.Felt {
	if utils.IsNil(v) {
		return nil
	}

	return new(felt.Felt).SetBytes(v.GetElements())
}

func AdaptUint128(u *gen.Uint128) *felt.Felt {
	if u == nil {
		return nil
	}

	bytes := make([]byte, 16) //nolint:mnd

	binary.BigEndian.PutUint64(bytes[:8], u.High)
	binary.BigEndian.PutUint64(bytes[8:], u.Low)

	return new(felt.Felt).SetBytes(bytes)
}
