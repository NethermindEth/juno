package p2p2core

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/receipt"
)

func AdaptHash(h *common.Hash) *felt.Felt {
	return adapt(h)
}

func AdaptAddress(h *common.Address) *felt.Felt {
	return adapt(h)
}

func AdaptEthAddress(h *receipt.EthereumAddress) ethcommon.Address {
	return ethcommon.BytesToAddress(h.Elements)
}

func AdaptFelt(f *common.Felt252) *felt.Felt {
	return adapt(f)
}

func adapt(v interface{ GetElements() []byte }) *felt.Felt {
	if utils.IsNil(v) {
		return nil
	}

	return new(felt.Felt).SetBytes(v.GetElements())
}

func AdaptUint128(u *common.Uint128) *felt.Felt {
	if u == nil {
		return nil
	}

	bytes := make([]byte, 16)

	binary.BigEndian.PutUint64(bytes[:8], u.High)
	binary.BigEndian.PutUint64(bytes[8:], u.Low)

	return new(felt.Felt).SetBytes(bytes)
}
