package blake2s

import (
	"slices"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"golang.org/x/crypto/blake2s"
)

// Following the same implementation behind
// https://github.com/starknet-io/types-rs/blob/main/crates/starknet-types-core/src/hash/blake2s.rs

func Blake2s[F felt.FeltLike](x, y *F) (felt.Hash, error) {
	return Blake2sArray(x, y)
}

func Blake2sArray[F felt.FeltLike](feltLikes ...*F) (felt.Hash, error) {
	// It is assumed that F follows the exact same memory layout as felt.Felt
	// Otherwise this code will fail
	felts := unsafe.Slice((**felt.Felt)(unsafe.Pointer(&feltLikes[0])), len(feltLikes))

	for _, f := range felts {
		println(f.String())
	}

	encoding := encodeFeltsToBytes(felts...)

	hasher, err := blake2s.New256(nil)
	if err != nil {
		return felt.Hash{}, err
	}

	_, err = hasher.Write(encoding)
	if err != nil {
		return felt.Hash{}, err
	}

	result := make([]byte, 0, 32)
	result = hasher.Sum(result)
	// Result is in big endian, turning into little endian
	slices.Reverse(result)

	return felt.FromBytes[felt.Hash](result), nil
}
