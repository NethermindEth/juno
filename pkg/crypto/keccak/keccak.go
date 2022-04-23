// Package keccak implements legacy Keccak secure hash functions.
//
// For a detailed specification see: https://keccak.team.
package keccak

import (
	"math/big"

	"golang.org/x/crypto/sha3"
)

// Digest250 returns the 256-bit Keccak hash of the data with the first
// 6 bits set to zero. Note that signature differs from standard Go
// hash functions in that the return type is a *big.Int meant to
// represent a field element, 𝔽ₚ. See the [hash function] section of the
// StarkNet documentation for details.
//
// [hash function]: https://docs.starknet.io/docs/Hashing/hash-functions#starknet-keccak
func Digest250(data []byte) *big.Int {
	digest := Digest256(data)
	digest[0] >>= 6 // Clear the last 6 bits.
	return new(big.Int).SetBytes(digest)
}

// Digest256 returns the 256-bit Keccak hash of the data.
func Digest256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}
