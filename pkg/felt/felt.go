package felt

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/NethermindEth/juno/pkg/crypto/weierstrass"
)

// p is the prime number 2²⁵¹ + 17·2¹⁹² + 1.
var p *big.Int

func init() {
	p = weierstrass.Stark().Params().P
}

// Felt represents the field element type used in Cairo. A Felt is a
// number in the the half-open range [0, p) where
// p = 2²⁵¹ + 17·2¹⁹² + 1. All the operations are made over the ring ℤp,
// which means all operations are computed modulo p. For more
// information see the [Cairo documentation section on field elements].
//
// [Cairo documentation section on field elements]: https://www.cairo-lang.org/docs/how_cairo_works/cairo_intro.html#field-elements
type Felt big.Int

// int casts f to a [big.Int].
func (f *Felt) int() *big.Int {
	return (*big.Int)(f)
}

func (f *Felt) reduce() {
	i := f.int()
	i.Mod(i, p)
}

// New allocates and returns a new Felt set to x.
func New(x int64) *Felt {
	f := new(Felt)
	i := f.int()
	i.SetInt64(x)
	f.reduce()
	return f
}

// Set sets z to x and returns z.
func (z *Felt) Set(x *Felt) *Felt {
	z.int().Set(x.int())
	return z
}

// SetString sets z to the value of s, interpreted in the given base,
// and returns z and a boolean indicating success. The entire string
// (not just a prefix) must be valid for success. If SetString fails,
// the value of z is undefined but the returned value is nil. The value
// is calculated modulo p to bring it to the valid range, [0, p).
func (z *Felt) SetString(s string, base int) (*Felt, bool) {
	_, ok := z.int().SetString(s, base)
	if !ok {
		return nil, ok
	}
	z.reduce()
	return z, true
}

// Add sets to z the sum x + y computed modulo p and returns z.
func (z *Felt) Add(x, y *Felt) *Felt {
	zi := z.int()
	zi.Add(x.int(), y.int())
	z.reduce()
	return z
}

// Sub sets to z the difference x - y computed modulo p and returns z.
func (z *Felt) Sub(x, y *Felt) *Felt {
	zi := z.int()
	zi.Sub(x.int(), y.int())
	z.reduce()
	return z
}

// Mul sets to z the product x * y computed modulo p and returns z.
func (z *Felt) Mul(x, y *Felt) *Felt {
	zi := z.int()
	zi.Mul(x.int(), y.int())
	z.reduce()
	return z
}

// Exp sets z = x**y mod |p|. For inputs of a particular size, this is
// not a cryptographically constant-time operation.
func (z *Felt) Exp(x, y *Felt) *Felt {
	zi := z.int()
	zi.Exp(x.int(), y.int(), p)
	return z
}

// Div sets to z the quotient x / y, with the notable difference that it
// has to satisfy the equation (x / y) * y == x in the ℤp ring i.e.
// x / y = (modInverse(y) * x) mod |p| where modInverse(y) = k such that
// yk ≡ 1 mod p.
func (z *Felt) Div(x, y *Felt) *Felt {
	z.int().Mul(x.int(), new(big.Int).ModInverse(y.int(), p))
	z.reduce()
	return z
}

// Cmp compares x and y and returns:
//
//   -1 if x <  y
//    0 if x == y
//   +1 if x >  y
//
func (x *Felt) Cmp(y *Felt) int {
	return x.int().Cmp(y.int())
}

// Text returns the string representation of x in the given base.
func (x *Felt) Text(base int) string {
	return x.int().Text(base)
}

// UnmarshalJSON implements the [json.Unmarshaler] interface for the
// Felt type. The expected data values are:
//
// 	- Hexadecimal string: "0x123abc123"
// 	- Decimal string: 		"123"
// 	- Decimal int: 				123
//
func (x *Felt) UnmarshalJSON(data []byte) error {
	// Data is a string.
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		// Try decode data as a hexadecimal number.
		if len(s) > 2 && s[:2] == "0x" {
			value, ok := new(big.Int).SetString(s[2:], 16)
			if !ok {
				return fmt.Errorf("felt: cannot unmarshal %q into a *Felt", data)
			}
			x.Set((*Felt)(value))
		} else {
			// Try decode data as a decimal number.
			value, err := strconv.Atoi(s)
			if err != nil {
				return err
			}
			x.Set(New(int64(value)))
		}
	} else {
		// Try to decode as an decimal number.
		var value int64
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		x.Set(New(value))
	}
	return nil
}

// String makes Felt conform to the [fmt.Stringer] interface.
func (x Felt) String() string {
	return x.int().Text(16)
}
