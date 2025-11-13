package crypto

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"golang.org/x/crypto/sha3"
)

// StarknetKeccak implements [Starknet keccak]
//
// [Starknet keccak]: https://docs.starknet.io/architecture-and-concepts/cryptography/hash-functions/#starknet_keccak
func StarknetKeccak(b []byte) felt.Felt {
	h := sha3.NewLegacyKeccak256()
	_, err := h.Write(b)
	if err != nil {
		// actual implementation (sha3.state{} type) doesn't return error in Write method
		// we keep this panic as an assertion in case they will modify the implementation
		panic(fmt.Errorf("failed to write to LegacyKeccak256 hash: %w", err))
	}
	d := h.Sum(nil)
	// Remove the first 6 bits from the first byte
	d[0] &= 3
	return felt.FromBytes[felt.Felt](d)
}
