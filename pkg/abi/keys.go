package abi

import (
	"encoding/binary"
	"math/big"
)

func newSimpleKey(contractAddress string) []byte {
	i, ok := new(big.Int).SetString(contractAddress, 16)
	if !ok {
		panic(any("invalid contractAddress string format, expected an hex string"))
	}
	return i.Bytes()
}

func newCompoundedKey(contractAddress string, blockNumber uint64) []byte {
	key := newSimpleKey(contractAddress)
	key = append(key, '.')
	bn := make([]byte, 8)
	binary.LittleEndian.PutUint64(bn, blockNumber)
	return append(key, bn...)
}
