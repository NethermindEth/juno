package types

import (
	"github.com/ethereum/go-ethereum/common"
)

func L1AddressFromEth(addr common.Address) L1Address {
	var result L1Address
	var padded [32]byte
	copy(padded[12:], addr[:])
	_ = result.SetBytesCanonical(padded[:])
	return result
}

func (a L1Address) ToEthAddress() common.Address {
	bytes := a.Bytes()
	var addr common.Address
	copy(addr[:], bytes[12:])
	return addr
}

func L1HashFromEth(hash common.Hash) L1Hash {
	var result L1Hash
	_ = result.SetBytesCanonical(hash[:])
	return result
}

func (h L1Hash) ToEthHash() common.Hash {
	bytes := h.Bytes()
	return common.BytesToHash(bytes[:])
}
