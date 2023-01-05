package felt

import (
	"errors"
	"math/big"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

type Felt struct {
	*fp.Element
}

var bigIntPool = sync.Pool{
	New: func() interface{} {
		return new(big.Int)
	},
}

// UnmarshalJSON accepts numbers and strings as input.
// See Element.SetString for valid prefixes (0x, 0b, ...).
// If there is an error, we try to explicitly unmarshal from hex before
// returning an error. This implementation is taken from [gnark-crypto].
//
// [gnark-crypto]: https://github.com/ConsenSys/gnark-crypto/blob/9fd0a7de2044f088a29cfac373da73d868230148/ecc/stark-curve/fp/element.go#L1028-L1056
func (z *Felt) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) > fp.Bits*3 {
		return errors.New("value too large (max = Element.Bits * 3)")
	}

	// we accept numbers and strings, remove leading and trailing quotes if any
	if len(s) > 0 && s[0] == '"' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}

	// get temporary big int from the pool
	vv := bigIntPool.Get().(*big.Int)

	if _, ok := vv.SetString(s, 0); !ok {
		if _, ok := vv.SetString(s, 16); !ok {
			return errors.New("can't parse into a big.Int: " + s)
		}
	}

	z.SetBigInt(vv)

	// release object into pool
	bigIntPool.Put(vv)
	return nil
}

func NewFelt(v uint64) Felt {
	return Felt{&fp.Element{v}}
}

func (z *Felt) SetUint64(v uint64) *Felt {
	return &Felt{z.Element.SetUint64(v)}
}

// SetInt64 sets z to v and returns z
func (z *Felt) SetInt64(v int64) *Felt {
	return &Felt{z.Element.SetInt64(v)}
}

func (z *Felt) Set(v *Felt) *Felt {
	return &Felt{z.Element.Set(v.Element)}
}

func (z *Felt) SetInterface(i1 interface{}) (*Felt, error) {
	vv, err := z.Element.SetInterface(i1)
	return &Felt{vv}, err
}

func (z *Felt) SetZero() *Felt {
	return &Felt{z.Element.SetZero()}
}

func (z *Felt) SetOne() *Felt {
	return &Felt{z.Element.SetOne()}
}

func (z *Felt) SetRandom() (*Felt, error) {
	vv, err := z.Element.SetRandom()
	return &Felt{vv}, err
}

func (z *Felt) SetBigInt(v *big.Int) *Felt {
	return &Felt{z.Element.SetBigInt(v)}
}

func (z *Felt) SetString(s string) (*Felt, error) {
	vv, err := z.Element.SetString(s)
	vFelt := Felt{vv}
	return &vFelt, err
}

func (z *Felt) SetBytes(b []byte) *Felt {
	return &Felt{z.Element.SetBytes(b)}
}

func (z *Felt) Div(x, y *Felt) *Felt {
	return &Felt{z.Element.Div(x.Element, y.Element)}
}

func (z *Felt) Bit(i uint64) uint64 {
	return z.Element.Bit(i)
}

func (z *Felt) Equal(x *Felt) bool {
	return z.Element.Equal(x.Element)
}

func (z *Felt) NotEqual(x *Felt) uint64 {
	return z.Element.NotEqual(x.Element)
}

func One() Felt {
	vv := fp.One()
	return Felt{&vv}
}

func (z *Felt) IsZero() bool {
	return z.Element.IsZero()
}

func (z *Felt) IsOne() bool {
	return z.Element.IsOne()
}

func (z *Felt) IsUint64() bool {
	return z.Element.IsUint64()
}

func (z *Felt) Uint64() uint64 {
	return z.Element.Uint64()
}

func (z *Felt) FitsOnOneWord() bool {
	return z.Element.FitsOnOneWord()
}

func (z *Felt) Cmp(x *Felt) int {
	return z.Element.Cmp(x.Element)
}

func (z *Felt) LexicographicallyLargest() bool {
	return z.Element.LexicographicallyLargest()
}

func (z *Felt) Add(x, y *Felt) *Felt {
	return &Felt{z.Element.Add(x.Element, y.Element)}
}

func (z *Felt) Sub(x, y *Felt) *Felt {
	return &Felt{z.Element.Sub(x.Element, y.Element)}
}

func (z *Felt) Mul(x, y *Felt) *Felt {
	return &Felt{z.Element.Mul(x.Element, y.Element)}
}

func (z *Felt) Neg(x *Felt) *Felt {
	return &Felt{z.Element.Neg(x.Element)}
}

func (z *Felt) Double(x *Felt) *Felt {
	return &Felt{z.Element.Double(x.Element)}
}

func (z *Felt) BitLen() int {
	return z.Element.BitLen()
}

func (z *Felt) Square(x *Felt) *Felt {
	return &Felt{z.Element.Square(x.Element)}
}

func (z *Felt) Sqrt(x *Felt) *Felt {
	return &Felt{z.Element.Sqrt(x.Element)}
}

func (z *Felt) Inverse(x *Felt) *Felt {
	return &Felt{z.Element.Inverse(x.Element)}
}

func (z *Felt) Exp(x *Felt, k *big.Int) *Felt {
	return &Felt{z.Element.Exp(*x.Element, k)}
}

func (z *Felt) Bytes() (res [fp.Limbs * 8]byte) {
	return z.Element.Bytes()
}

func (z *Felt) FromMont() *Felt {
	return &Felt{z.Element.FromMont()}
}

func (z *Felt) Select(c int, x0 *Felt, x1 *Felt) *Felt {
	return &Felt{z.Element.Select(c, x0.Element, x1.Element)}
}

func (z *Felt) ToMont() *Felt {
	return &Felt{z.Element.ToMont()}
}

func (z *Felt) ToRegular() *Felt {
	vv := z.Element.ToRegular()
	return &Felt{&vv}
}

func (z *Felt) String() string {
	return z.Element.String()
}

func (z *Felt) Text(base int) string {
	return z.Element.Text(base)
}

func (z *Felt) ToBigInt(res *big.Int) *big.Int {
	return z.Element.ToBigInt(res)
}

func (z *Felt) ToBigIntRegular(res *big.Int) *big.Int {
	return z.Element.ToBigIntRegular(res)
}

func (z *Felt) Marshal() []byte {
	return z.Element.Marshal()
}

func (z *Felt) MarshalJSON() ([]byte, error) {
	return z.Element.MarshalJSON()
}

func (z *Felt) Legendre() int {
	return z.Element.Legendre()
}
