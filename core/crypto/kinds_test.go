package crypto_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/crypto/blake2s"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
)

// The felt type only changes what callers can pass and get back, never the hash value.
func TestHashesIgnoreFeltKinds(t *testing.T) {
	address := felt.Random[felt.Address]()
	classHash := felt.Random[felt.ClassHash]()
	hashes := []felt.Hash{felt.Random[felt.Hash](), felt.Random[felt.Hash](), felt.Random[felt.Hash]()}

	plainAddress := felt.Felt(address)
	plainClassHash := felt.Felt(classHash)
	plainHashes := felt.CastSlice[felt.Felt](hashes)
	hashPointers := []*felt.Hash{&hashes[0], &hashes[1], &hashes[2]}
	plainPointers := []*felt.Felt{&plainHashes[0], &plainHashes[1], &plainHashes[2]}

	t.Run("pair", func(t *testing.T) {
		assert.Equal(t,
			felt.Hash(crypto.Pedersen[felt.Felt](&plainAddress, &plainClassHash)),
			crypto.Pedersen[felt.Hash](&address, &classHash),
		)
		assert.Equal(t,
			felt.Hash(crypto.Poseidon[felt.Felt](&plainAddress, &plainClassHash)),
			crypto.Poseidon[felt.Hash](&address, &classHash),
		)
	})

	t.Run("array", func(t *testing.T) {
		assert.Equal(t,
			felt.StateRootHash(crypto.PedersenArray[felt.Felt](plainHashes)),
			crypto.PedersenArray[felt.StateRootHash](hashes),
		)
		assert.Equal(t,
			felt.StateRootHash(crypto.PoseidonArray[felt.Felt](plainHashes)),
			crypto.PoseidonArray[felt.StateRootHash](hashes),
		)
		assert.Equal(t,
			felt.StateRootHash(blake2s.Blake2sArray[felt.Felt](plainHashes)),
			blake2s.Blake2sArray[felt.StateRootHash](hashes),
		)
	})

	t.Run("elems", func(t *testing.T) {
		assert.Equal(t,
			felt.Hash(crypto.PedersenElems[felt.Felt](plainPointers...)),
			crypto.PedersenElems[felt.Hash](hashPointers...),
		)
		assert.Equal(t,
			felt.Hash(crypto.PoseidonElems[felt.Felt](plainPointers...)),
			crypto.PoseidonElems[felt.Hash](hashPointers...),
		)
	})

	t.Run("pedersen digest with mixed kinds", func(t *testing.T) {
		var plain, mixed crypto.PedersenDigest
		plain.Update(&plainAddress).Update(&plainClassHash).UpdateArray(plainHashes)
		mixed.Update(&address).Update(&classHash).UpdateArray(hashes)
		assert.Equal(t, felt.Hash(plain.Finish[felt.Felt]()), mixed.Finish[felt.Hash]())
	})

	t.Run("poseidon digest with mixed kinds", func(t *testing.T) {
		var plain, mixed crypto.PoseidonDigest
		plain.Update(&plainAddress).Update(&plainClassHash).UpdateArray(plainHashes)
		mixed.Update(&address).Update(&classHash).UpdateArray(hashes)
		assert.Equal(t, felt.Hash(plain.Finish[felt.Felt]()), mixed.Finish[felt.Hash]())
	})

	t.Run("blake2s digest with mixed kinds", func(t *testing.T) {
		var plain, mixed blake2s.Blake2sDigest
		plain.Update(&plainAddress).Update(&plainClassHash).UpdateArray(plainHashes)
		mixed.Update(&address).Update(&classHash).UpdateArray(hashes)
		assert.Equal(t, felt.Hash(plain.Finish[felt.Felt]()), mixed.Finish[felt.Hash]())
	})
}
