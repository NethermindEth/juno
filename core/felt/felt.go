package felt

import (
	"errors"
	"math/big"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

type Felt struct {
	val fp.Element
}

func NewFelt(element *fp.Element) *Felt {
	return &Felt{
		val: *element,
	}
}

const (
	Limbs = fp.Limbs // number of 64 bits words needed to represent a Element
	Bits  = fp.Bits  // number of bits needed to represent a Element
	Bytes = fp.Bytes // number of bytes needed to represent a Element
)

// zero felt constant
var Zero = Felt{}

var bigIntPool = sync.Pool{
	New: func() interface{} {
		return new(big.Int)
	},
}

// Impl returns the underlying field element type
func (z *Felt) Impl() *fp.Element {
	return &z.val
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

	z.val.SetBigInt(vv)

	// release object into pool
	bigIntPool.Put(vv)
	return nil
}

// MarshalJSON forwards the call to underlying field element implementation
func (z *Felt) MarshalJSON() ([]byte, error) {
	return z.val.MarshalJSON()
}

// SetInterface forwards the call to underlying field element implementation
func (z *Felt) SetInterface(i1 interface{}) (*Felt, error) {
	_, err := z.val.SetInterface(i1)
	return z, err
}

// SetBytes forwards the call to underlying field element implementation
func (z *Felt) SetBytes(e []byte) *Felt {
	z.val.SetBytes(e)
	return z
}

// SetString forwards the call to underlying field element implementation
func (z *Felt) SetString(number string) (*Felt, error) {
	_, err := z.val.SetString(number)
	return z, err
}

// SetUint64 forwards the call to underlying field element implementation
func (z *Felt) SetUint64(v uint64) *Felt {
	z.val.SetUint64(v)
	return z
}

// SetRandom forwards the call to underlying field element implementation
func (z *Felt) SetRandom() (*Felt, error) {
	_, err := z.val.SetRandom()
	return z, err
}

// String forwards the call to underlying field element implementation
func (z *Felt) String() string {
	return z.val.String()
}

// Text forwards the call to underlying field element implementation
func (z *Felt) Text(base int) string {
	return z.val.Text(base)
}

// Equal forwards the call to underlying field element implementation
func (z *Felt) Equal(x *Felt) bool {
	return z.val.Equal(&x.val)
}

// Marshal forwards the call to underlying field element implementation
func (z *Felt) Marshal() []byte {
	return z.val.Marshal()
}

// Bytes forwards the call to underlying field element implementation
func (z *Felt) Bytes() [32]byte {
	return z.val.Bytes()
}

// ToRegular forwards the call to underlying field element implementation
func (z Felt) ToRegular() Felt {
	z.val = z.val.ToRegular()
	return z
}

// IsOne forwards the call to underlying field element implementation
func (z *Felt) IsOne() bool {
	return z.val.IsOne()
}

// IsZero forwards the call to underlying field element implementation
func (z *Felt) IsZero() bool {
	return z.val.IsZero()
}

// Bit forwards the call to underlying field element implementation
func (z *Felt) Bit(i uint64) uint64 {
	return z.val.Bit(i)
}

// Add forwards the call to underlying field element implementation
func (z *Felt) Add(x, y *Felt) *Felt {
	z.val.Add(&x.val, &y.val)
	return z
}

// Halve forwards the call to underlying field element implementation
func (z *Felt) Halve() {
	z.val.Halve()
}

// Cmp forwards the call to underlying field element implementation
func (z *Felt) Cmp(x *Felt) int {
	return z.val.Cmp(&x.val)
}
