// Package pedersen implements the Starknet variant of the Pedersen
// hash function.
package pedersen

import (
	_ "embed"
	"fmt"
	"math/big"
)

// Digest returns a field element that is the result of hashing an input
// (a, b) ∈ 𝔽²ₚ where p = 2²⁵¹ + 17·2¹⁹² + 1. This function will panic
// if len(data) > 2. In order to hash n > 2 items, use [ArrayDigest].
func Digest(data ...*big.Int) *big.Int {
	n := len(data)
	if n > 2 {
		panic("attempted to hash more than 2 field elements")
	}

	// Make a defensive copy of the input data.
	elements := make([]*big.Int, n)
	for i, e := range data {
		elements[i] = new(big.Int).Set(e)
	}

	// Shift point.
	pt1 := points[0]
	zero := new(big.Int)
	for i, x := range elements {
		if !(x.Cmp(zero) != -1 && curve.Params().P.Cmp(x) == 1) {
			panic(fmt.Sprintf("%x is not in the range 0 < x < 2²⁵¹ + 17·2¹⁹² + 1", x))
		}

		for j := 0; j < 252; j++ {
			pt2 := points[2+i*252+j]
			// Create a copy because *big.Int.And mutates.
			copy := new(big.Int).Set(x)
			if copy.And(copy, big.NewInt(1)).Cmp(big.NewInt(0)) != 0 {
				x1, x2 := curve.Add(pt1.x, pt1.y, pt2.x, pt2.y)
				pt1 = point{x1, x2}
			}
			x.Rsh(x, 1)
		}
	}
	return pt1.x
}

// ArrayDigest returns a field element that is the result of hashing an
// array of field elements. This is generally used to overcome the
// limitation of the [Digest] function which has an upper bound on the
// amount of field elements that can be hashed. See the [array hashing]
// section of the StarkNet documentation for more details.
//
// [array hashing]: https://docs.starknet.io/docs/Hashing/hash-functions#array-hashing
func ArrayDigest(data ...*big.Int) *big.Int {
	digest := new(big.Int)
	for _, item := range data {
		digest = Digest(digest, item)
	}
	return Digest(digest, big.NewInt(int64(len(data))))
}
