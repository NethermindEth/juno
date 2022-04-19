package types

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/NethermindEth/juno/pkg/crypto/weierstrass"
)

var (
	p *big.Int
	// pHalf *big.Int
)

func init() {
	curve := weierstrass.Stark()
	p = curve.Params().P
	// pHalf = new(big.Int).Div(p, big.NewInt(2))
}

// Felt type is the representation of the field element type of Cairo. A Felt is
// a number that is in the range [0,p) where p = 2²⁵¹ + 17·2¹⁹² + 1. All the
// operations with Felt are made over de ring ℤp, that's means all operations
// are made modulo p.
//
//See https://www.cairo-lang.org/docs/hello_cairo/intro.html#the-primitive-type-field-element-felt
type Felt big.Int

// asInt cast f as a big.Int
func (f *Felt) asInt() *big.Int {
	return (*big.Int)(f)
}

func (f *Felt) reduce() {
	fV := f.asInt()
	// neg := fV.Sign()
	// fV.Abs(fV)
	fV.Mod(fV, p)
	// if fV.Cmp(pHalf) == 1 {
	// 	fV.Sub(fV, p)
	// }
	// if neg == -1 {
	// 	fV.Neg(fV)
	// }
}

// NewInt create a new Felt from an int64
func NewInt(x int64) *Felt {
	felt := new(Felt)
	feltV := felt.asInt()
	feltV.SetInt64(x)
	felt.reduce()
	return felt
}

// Set sets z to x and returns z
func (z *Felt) Set(x *Felt) *Felt {
	z.asInt().Set(x.asInt())
	return z
}

// SetString sets z to the value of s, interpreted in the given base,
// and returns z and a boolean indicating success. The entire string
// (not just a prefix) must be valid for success. If SetString fails,
// the value of z is undefined but the returned value is nil.
//
// The value is calculated modulo p to bring it to the valid range [0,p)
func (z *Felt) SetString(s string, base int) (*Felt, bool) {
	_, ok := z.asInt().SetString(s, base)
	if !ok {
		return nil, ok
	}
	z.reduce()
	return z, true
}

// Add sets to z the sum of x + y in modular arithmetic with modulo p and
// returns z.
//
// z = (x + y) mod |p|
func (z *Felt) Add(x, y *Felt) *Felt {
	zV := z.asInt()
	xV := x.asInt()
	yV := y.asInt()
	zV.Add(xV, yV)
	z.reduce()
	return z
}

// Sub sets to z the difference of x - y in modular arithmetic with modulo p and
// returns z.
//
// z = (x - y) mod |p|
func (z *Felt) Sub(x, y *Felt) *Felt {
	zV := z.asInt()
	xV := x.asInt()
	yV := y.asInt()
	zV.Sub(xV, yV)
	z.reduce()
	return z
}

// Mul sets to z the product of x * y in modular arithmetic with modulo p and
// returns z.
//
// z = (x * y) mod |p|
func (z *Felt) Mul(x, y *Felt) *Felt {
	zV := z.asInt()
	xV := x.asInt()
	yV := y.asInt()
	zV.Mul(xV, yV)
	z.reduce()
	return z
}

// Exp sets z = x**y mod |p|
func (z *Felt) Exp(x, y *Felt) *Felt {
	zV := z.asInt()
	xV := x.asInt()
	yV := y.asInt()
	zV.Exp(xV, yV, p)
	return z
}

// Div sets to z the division of x / y, where de division is not the integer
// division beacuse the division must be achieve with the restriction (x/y)*y=x
// inside the ℤp ring.
//
// x / y = (modInverse(y) * x) mod |p|
//
// Where modInverse(y) = k souch that yk ≡ 1 mod p
func (z *Felt) Div(x, y *Felt) *Felt {
	zV := z.asInt()
	xV := x.asInt()
	yV := y.asInt()
	u := new(big.Int).ModInverse(yV, p)
	zV.Mul(xV, u)
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
	xV := x.asInt()
	yV := y.asInt()
	return xV.Cmp(yV)
}

// Text returns the string representation of x in the given base.
func (x *Felt) Text(base int) string {
	xV := x.asInt()
	return xV.Text(base)
}

// UnmarshalJSON unmarshals the data into a Felt type. The expected data types
// are:
//
// 	- Hexadecimal string: "0x123abc123"
// 	- Decimal string: "123"
// 	- Decimal int: 123
func (x *Felt) UnmarshalJSON(data []byte) error {
	// Data is a string
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		// Try decode data as a hexadecimal number
		if len(s) > 2 && s[:2] == "0x" {
			value, ok := new(big.Int).SetString(s[2:], 16)
			if !ok {
				return fmt.Errorf("error unmarshaling hexadecimal string into Felt")
			}
			x.Set((*Felt)(value))
		} else {
			// Try decode data as a decimal number
			value, err := strconv.Atoi(s)
			if err != nil {
				return err
			}
			x.Set(NewInt(int64(value)))
		}
	} else {
		// Try to decode as an decimal number
		var value int64
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		x.Set(NewInt(value))
	}
	return nil
}

func (x Felt) String() string {
	return "0x" + x.Text(16)
}
