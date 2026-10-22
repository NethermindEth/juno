package felt

import "github.com/consensys/gnark-crypto/ecc/stark-curve/fp"

// IsZero reports whether v is zero, as the field element implementation defines it.
func IsZero[F FeltLike](v *F) bool {
	element := fp.Element(*v)
	return element.IsZero()
}

// Equal reports whether a and b hold the same value, as the field element defines it.
func Equal[F FeltLike](a, b *F) bool {
	x, y := fp.Element(*a), fp.Element(*b)
	return x.Equal(&y)
}
