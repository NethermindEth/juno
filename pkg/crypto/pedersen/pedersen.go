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

// NOTE: The upper bound on the number of `*big.Int`s that can hashed in
// a single call (≤ 2) is not enforced for now.

// Digest returns a field element that is the result of hashing an input
// (a, b) ∈ 𝔽²ₚ where p = 2²⁵¹ + 17·2¹⁹² + 1.
func Digest(data ...*big.Int) (*big.Int, error) {
	// Make a defensive copy of the input data.
	elements := make([]*big.Int, len(data))
	for i, e := range data {
		elements[i] = new(big.Int).Set(e)
	}
	pt1 := points[0] // Shift point.
	for i, x := range elements {
		if !(x.Cmp(zero) != -1 && x.Cmp(curve.Params().P) == -1) {
			// notest
			// x is not in the range 0 < x < 2²⁵¹ + 17·2¹⁹² + 1.
			return zero, ErrInvalid
		}

		for j := 0; j < 252; j++ {
			pt2 := points[2+i*252+j]
			if pt1.x.Cmp(pt2.x) == 0 {
				// notest
				// Input cannot be hashed.
				return zero, ErrInvalid
			}
			cp := new(big.Int).Set(x) // Copy because *big.Int.And mutates.
			if cp.And(cp, big.NewInt(1)).Cmp(big.NewInt(0)) != 0 {
				x1, x2 := curve.Add(pt1.x, pt1.y, pt2.x, pt2.y)
				pt1 = point{x1, x2}
			}
			x.Rsh(x, 1)
		}
	}
	return pt1.x, nil
}

func DigestArray(data ...*big.Int) (*big.Int, error) {
	n := len(data)

	currentHash := zero

	for _, item := range data {
		partialResult, err := Digest(currentHash, item)
		if err != nil {
			return zero, err
		}
		currentHash = partialResult
	}

	return Digest(currentHash, new(big.Int).SetInt64(int64(n)))
}
