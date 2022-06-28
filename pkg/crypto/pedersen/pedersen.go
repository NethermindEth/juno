// Package pedersen implements the StarkNet variant of the Pedersen
// hash function.
package pedersen

import (
	_ "embed"

	"github.com/NethermindEth/juno/pkg/felt"
)

// Digest returns a field element that is the result of hashing an input
// (a, b) ∈ 𝔽²ₚ where p = 2²⁵¹ + 17·2¹⁹² + 1. This function will panic
// if len(data) > 2. In order to hash n > 2 items, use ArrayDigest.
func Digest(data ...*felt.Felt) *felt.Felt {
	n := len(data)
	if n > 2 {
		panic("attempted to hash more than 2 field elements")
	}

	// Make a defensive copy of the input data.
	elements := make([]*felt.Felt, n)
	for i, e := range data {
		elements[i] = new(felt.Felt).Set(e)
	}

	// Shift point.
	pt1 := points[0]
	for i, x := range elements {
		for j := 0; j < 252; j++ {
			if x.FromMont().Bit(0) != 0 {
				// x is odd
				pt1.Add(&points[2+i*252+j])
			}
			x.ToMont()
			x.Rsh(x, 1) // Can't use halve because we don't want mod P
		}
	}

	return pt1.x
}

// ArrayDigest returns a field element that is the result of hashing an
// array of field elements. This is generally used to overcome the
// limitation of the Digest function which has an upper bound on the
// amount of field elements that can be hashed. See the array hashing
// section of the StarkNet documentation https://docs.starknet.io/docs/Hashing/hash-functions#array-hashing
// for more details.
func ArrayDigest(data ...*felt.Felt) *felt.Felt {
	digest := new(felt.Felt)
	for _, item := range data {
		digest = Digest(digest, item)
	}
	return Digest(digest, new(felt.Felt).SetUint64(uint64(len(data))))
}
