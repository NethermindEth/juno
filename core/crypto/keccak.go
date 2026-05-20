package crypto

import (
	"fmt"
	"hash"

	"github.com/NethermindEth/juno/core/felt"
	"golang.org/x/crypto/sha3"
)

// NewStarknetKeccakState returns a fresh Keccak-256 state that callers can
// stream bytes into. Finalize with StarknetKeccakSum to obtain the felt.
func NewStarknetKeccakState() hash.Hash {
	return sha3.NewLegacyKeccak256()
}

// StarknetKeccakSum finalizes a Keccak-256 state into a Starknet felt by
// masking the top 6 bits of the digest and reducing modulo the felt prime.
func StarknetKeccakSum(h hash.Hash) felt.Felt {
	d := h.Sum(nil)
	d[0] &= 3
	return felt.FromBytes[felt.Felt](d)
}

// StarknetKeccak implements [Starknet keccak]
//
// [Starknet keccak]: https://docs.starknet.io/architecture-and-concepts/cryptography/hash-functions/#starknet_keccak
func StarknetKeccak(b []byte) felt.Felt {
	h := NewStarknetKeccakState()
	if _, err := h.Write(b); err != nil {
		// actual implementation (sha3.state{} type) doesn't return error in Write method
		// we keep this panic as an assertion in case they will modify the implementation
		panic(fmt.Errorf("failed to write to LegacyKeccak256 hash: %w", err))
	}
	return StarknetKeccakSum(h)
}
