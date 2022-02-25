package internal

import (
	"github.com/ethereum/go-ethereum/crypto"
	//log "github.com/sirupsen/logrus"
)

func SnKeccak(data []byte) []byte {
	logger.With("Data", data).Debug("Getting SN_Keccak")
	b := crypto.Keccak256(data)
	// Extract most significant byte to bitmask last 6 bits
	msb := b[0]
	// Bitmask with 0000 0011 to remove last 6 bits
	bitmask := uint8(3)
	newMsb := []byte{msb & bitmask}
	// Concatenate new most significant byte with remaining 31 bytes
	res := append(newMsb, b[1:]...)
	return res
}
