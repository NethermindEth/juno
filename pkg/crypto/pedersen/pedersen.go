// Package pedersen implements the Starknet variant of the Pedersen
// hash function.
package pedersen

import (
	_ "embed"
	"fmt"
	"math/big"
)

// See https://github.com/dontpanicdao/caigo/blob/e34006317632c87334fa29e770a00b5c7c94cb20/curve.go#L339-L351
func divMod(n, m, p *big.Int) *big.Int {
	q := new(big.Int)
	gx := new(big.Int)
	gy := new(big.Int)
	q.GCD(gx, gy, m, p)

	r := new(big.Int).Mul(n, gx)
	r = r.Mod(r, p)
	return r
}

// Add returns the sum of (x1, y1) and (x2, y2).
// This is based on the implementation in caigo
// See https://github.com/dontpanicdao/caigo/blob/e34006317632c87334fa29e770a00b5c7c94cb20/curve.go#L160-L182
func add(
	x1, y1, x2, y2 *big.Int,
) (x, y *big.Int) {
	xDelta := new(big.Int).Sub(x1, x2)
	yDelta := new(big.Int).Sub(y1, y2)

	m := divMod(yDelta, xDelta, p)
	x = new(big.Int).Mul(m, m)
	x.Sub(x, x1)
	x.Sub(x, x2)
	x.Mod(x, p)

	y = new(big.Int).Sub(x1, x)
	y.Mul(m, y)
	y.Sub(y, y1)
	y.Mod(y, p)

	if x1.Sign() == 0 {
		x.Set(x2)
	} else if x2.Sign() == 0 {
		x.Set(x1)
	}

	if y1.Sign() == 0 {
		y.Set(y2)
	} else if y2.Sign() == 0 {
		y.Set(y1)
	}

	return x, y
}

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
		if x.Cmp(zero) == -1 || x.Cmp(p) == 1 {
			panic(fmt.Sprintf("%x is not in the range 0 < x < 2²⁵¹ + 17·2¹⁹² + 1", x))
		}

		for j := 0; j < 252; j++ {
			pt2 := points[2+i*252+j]
			// Create a copy because *big.Int.And mutates.
			copy := new(big.Int).Set(x)
			if copy.And(copy, big.NewInt(1)).Cmp(big.NewInt(0)) != 0 {
				x1, x2 := add(pt1.x, pt1.y, pt2.x, pt2.y)
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
