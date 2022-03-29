// Package keccak implements legacy Keccak secure hash functions.
//
// For a detailed specification see: https://keccak.team.
package keccak

import "golang.org/x/crypto/sha3"

// Digest250 returns the 256-bit Keccak hash with the last 6 bits set to
// zero, as a 32 byte array. This is done so that the output fits inside
// of a single felt.
func Digest250(data []byte) []byte {
	dgst := Digest256(data)
	dgst[0] >>= 6 // Clear the last 6 bits.
	return dgst
}

// Digest256 returns the 256-bit Keccak hash of the data as a slice of
// 32 bytes.
func Digest256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}
