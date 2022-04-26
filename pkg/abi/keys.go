package abi

import "encoding/binary"

func newSimpleKey(contractAddress string) []byte {
	return []byte(contractAddress)
}

func newCompoundedKey(contractAddress string, blockNumber uint64) []byte {
	key := newSimpleKey(contractAddress)
	key = append(key, '.')
	bn := make([]byte, 8)
	binary.LittleEndian.PutUint64(bn, blockNumber)
	return append(key, bn...)
}
