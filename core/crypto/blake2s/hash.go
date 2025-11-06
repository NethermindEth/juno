package blake2s

import (
	"slices"
	"unsafe"

	"github.com/NethermindEth/juno/core/felt"
	"golang.org/x/crypto/blake2s"
)

// Following the same implementation behind
// https://github.com/starknet-io/types-rs/blob/main/crates/starknet-types-core/src/hash/blake2s.rs

func Blake2s[F felt.FeltLike](x, y *F) felt.Hash {
	return Blake2sArray(x, y)
}

func Blake2sArray[F felt.FeltLike](feltLikes ...*F) felt.Hash {
	var felts []*felt.Felt
	if len(feltLikes) > 0 {
		// It is assumed that type F follows the exact same memory layout as felt.Felt
		felts = unsafe.Slice((**felt.Felt)(unsafe.Pointer(&feltLikes[0])), len(feltLikes))
	} else {
		felts = []*felt.Felt{}
	}

	encoding := encodeFeltsToBytes(felts...)

	// errors if initialised with more than 32 bytes
	hasher, err := blake2s.New256(nil)
	if err != nil {
		panic(err)
	}

	// implementation does not errors, here it complies with `io.Writer`
	_, err = hasher.Write(encoding)
	if err != nil {
		panic(err)
	}

	result := make([]byte, 0, 32)
	result = hasher.Sum(result)
	// Result is in big endian, turning into little endian
	slices.Reverse(result)

	return felt.FromBytes[felt.Hash](result)
}
