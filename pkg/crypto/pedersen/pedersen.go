// Package pedersen implements the Starknet variant of the Pedersen
// hash function.
package pedersen

import (
	_ "embed"
	"encoding/json"
	"errors"
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/weierstrass"
)

// point represents the affine coordinates of an elliptic curve point.
type point struct{ x, y *big.Int }

var (
	// b is a byte array that represents the file that contains the
	// constant points, points.json.
	//go:embed points.json
	b []byte
	// points is a slice of *big.Int that contains the constant points in
	// the points.json file.
	points [506]point
	// curve is the elliptic (STARK) curve used to compute the Pedersen
	// hash.
	curve weierstrass.Curve
	// zero is a *big.Int that represents the constant 0.
	zero *big.Int
	// ErrInvalid indicates an input value that is not a field element
	// with p = 2²⁵¹ + 17·2¹⁹² + 1.
	ErrInvalid = errors.New("invalid argument")
)

func init() {
	var hex [506][2]string
	json.Unmarshal(b, &hex)
	for i, p := range hex {
		x, _ := new(big.Int).SetString(p[0], 16)
		y, _ := new(big.Int).SetString(p[1], 16)
		points[i] = point{x, y}
	}
	curve = weierstrass.Stark()
	zero = big.NewInt(0)
}

// Digest returns a field element that is the result of hashing an input
// (a, b) ∈ 𝔽²ₚ where p = 2²⁵¹ + 17·2¹⁹² + 1. This function will panic
// if len(data) > 2 and return an error if (a, b) ∉ 𝔽²ₚ or the point at
// infinity.
func Digest(data ...*big.Int) (*big.Int, error) {
	n := len(data)
	if n > 2 {
		panic("attempted to hash more than 2 field elements")
	}
	// Make a defensive copy of the input data.
	elements := make([]*big.Int, n)
	for i, e := range data {
		elements[i] = new(big.Int).Set(e)
	}
	pt1 := points[0] // Shift point.
	for i, x := range elements {
		if !(x.Cmp(zero) != -1 && x.Cmp(curve.Params().P) == -1) {
			// notest
			// x is not in the range 0 < x < 2²⁵¹ + 17·2¹⁹² + 1.
			return nil, ErrInvalid
		}

		for j := 0; j < 252; j++ {
			pt2 := points[2+i*252+j]
			if pt1.x.Cmp(pt2.x) == 0 {
				// notest
				// Input cannot be hashed.
				return nil, ErrInvalid
			}
			copy := new(big.Int).Set(x) // Copy because *big.Int.And mutates.
			if copy.And(copy, big.NewInt(1)).Cmp(big.NewInt(0)) != 0 {
				x1, x2 := curve.Add(pt1.x, pt1.y, pt2.x, pt2.y)
				pt1 = point{x1, x2}
			}
			x.Rsh(x, 1)
		}
	}
	return pt1.x, nil
}

// ArrayDigest returns a field element that is the result of hashing an
// array of field elements. This is generally used to overcome the
// limitation of the [Digest] function which has an upper bound on the
// amount of field elements that can be hashed. See the [array hashing]
// section of the StarkNet documentation for more details.
//
// [array hashing]: https://docs.starknet.io/docs/Hashing/hash-functions#array-hashing
func ArrayDigest(data ...*big.Int) (*big.Int, error) {
	n := len(data)

	currentHash := zero

	for _, item := range data {
		partialResult, err := Digest(currentHash, item)
		if err != nil {
			return nil, err
		}
		currentHash = partialResult
	}

	return Digest(currentHash, big.NewInt(int64(n)))
}
