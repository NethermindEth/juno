package internal

import "github.com/ethereum/go-ethereum/crypto"

func SnKeccak(data []byte) []byte {
	b := crypto.Keccak256(data)
	// Extract most significant byte to bitmask last 6 bits
	msb := b[0]
	// Bitmask with 0000 0011 to remove last 6 bits
	bitmask := uint8(3)
	new_msb := []byte{msb & bitmask}
	// Concatenate new most significant byte with remaining 31 bytes
	res := append(new_msb, b[1:]...)
	return res
}
