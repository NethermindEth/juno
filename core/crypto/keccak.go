package crypto

import (
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"golang.org/x/crypto/sha3"
)

var h = sha3.NewLegacyKeccak256()

// StarkNetKeccak implements [StarkNet keccak]
//
// [StarkNet keccak]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#starknet_keccak
func StarkNetKeccak(b []byte) (*fp.Element, error) {
	h.Reset()
	_, err := h.Write(b)
	if err != nil {
		return nil, err
	}
	d := h.Sum(nil)
	// Remove the first 6 bits from the first byte
	d[0] &= 3
	return new(fp.Element).SetBytes(d), nil
}
