package felt

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/fxamacker/cbor/v2"
)

const (
	Base16 = 16
	Base10 = 10
)

const (
	Limbs = fp.Limbs // number of 64 bits words needed to represent a Element
	Bits  = fp.Bits  // number of bits needed to represent a Element
	Bytes = fp.Bytes // number of bytes needed to represent a Element
)

var Zero = Felt{}

var One = Felt(fp.Element(
	[4]uint64{
		18446744073709551585,
		18446744073709551615,
		18446744073709551615,
		576460752303422960,
	},
))

var bigIntPool = sync.Pool{
	New: func() any {
		return new(big.Int)
	},
}

type Felt fp.Element

// Impl returns the underlying field element type
func (z *Felt) Impl() *fp.Element {
	return (*fp.Element)(z)
}

// UnmarshalJSON accepts numbers and strings as input.
// See Element.SetString for valid prefixes (0x, 0b, ...).
// If there is an error, we try to explicitly unmarshal from hex before
// returning an error. This implementation is based on [gnark-crypto]'s UnmarshalJSON.
//
// [gnark-crypto]: https://github.com/ConsenSys/gnark-crypto/blob/master/ecc/stark-curve/fp/element.go
func (z *Felt) UnmarshalJSON(data []byte) error {
	if len(data) > fp.Bits*3 {
		return errors.New("value too large (max = Element.Bits * 3)")
	}

	// we accept numbers and strings, remove leading and trailing quotes if any
	if len(data) > 0 && data[0] == '"' {
		data = data[1:]
	}
	if len(data) > 0 && data[len(data)-1] == '"' {
		data = data[:len(data)-1]
	}

	_, err := z.SetString(string(data))
	return err
}

// MarshalJSON forwards the call to underlying field element implementation
func (z *Felt) MarshalJSON() ([]byte, error) {
	return []byte("\"" + z.String() + "\""), nil
}

// SetBytes forwards the call to underlying field element implementation
func (z *Felt) SetBytes(e []byte) *Felt {
	(*fp.Element)(z).SetBytes(e)
	return z
}

// SetBytesCanonical forwards the call to underlying field element implementation
func (z *Felt) SetBytesCanonical(e []byte) error {
	return (*fp.Element)(z).SetBytesCanonical(e)
}

// SetString forwards the call to underlying field element implementation
func (z *Felt) SetString(number string) (*Felt, error) {
	// get temporary big int from the pool
	vv := bigIntPool.Get().(*big.Int)
	// release object into pool
	defer bigIntPool.Put(vv)

	if _, ok := vv.SetString(number, 0); !ok {
		if _, ok := vv.SetString(number, Base16); !ok {
			return z, errors.New("can't parse into a big.Int: " + number)
		}
	}

	if vv.BitLen() > fp.Bits {
		return z, errors.New("can't fit in felt: " + number)
	}

	var bytes [32]byte
	vv.FillBytes(bytes[:])
	return z, (*fp.Element)(z).SetBytesCanonical(bytes[:])
}

// SetUint64 forwards the call to underlying field element implementation
func (z *Felt) SetUint64(v uint64) *Felt {
	(*fp.Element)(z).SetUint64(v)
	return z
}

// SetRandom forwards the call to underlying field element implementation
func (z *Felt) SetRandom() *Felt {
	_, err := (*fp.Element)(z).SetRandom()
	if err != nil {
		panic(fmt.Sprintf("unexpected error from rand.Reader: %s", err.Error()))
	}
	return z
}

// String forwards the call to underlying field element implementation
func (z *Felt) String() string {
	return "0x" + (*fp.Element)(z).Text(Base16)
}

// ShortString prints the felt to a string in a shortened format
func (z *Felt) ShortString() string {
	shortFelt := 8
	hex := (*fp.Element)(z).Text(Base16)

	if len(hex) <= shortFelt {
		return fmt.Sprintf("0x%s", hex)
	}
	return fmt.Sprintf("0x%s...%s", hex[:4], hex[len(hex)-4:])
}

// Text forwards the call to underlying field element implementation
func (z *Felt) Text(base int) string {
	return (*fp.Element)(z).Text(base)
}

// Equal forwards the call to underlying field element implementation
func (z *Felt) Equal(x *Felt) bool {
	return (*fp.Element)(z).Equal((*fp.Element)(x))
}

// Marshal forwards the call to underlying field element implementation
func (z *Felt) Marshal() []byte {
	return (*fp.Element)(z).Marshal()
}

// Unmarshal forwards the call to underlying field element implementation
func (z *Felt) Unmarshal(e []byte) {
	(*fp.Element)(z).Unmarshal(e)
}

// Bytes forwards the call to underlying field element implementation.
// Returns the value of z as a big-endian byte array
func (z *Felt) Bytes() [32]byte {
	return (*fp.Element)(z).Bytes()
}

// IsOne forwards the call to underlying field element implementation
func (z *Felt) IsOne() bool {
	return (*fp.Element)(z).IsOne()
}

// IsZero forwards the call to underlying field element implementation
func (z *Felt) IsZero() bool {
	return (*fp.Element)(z).IsZero()
}

// Add forwards the call to underlying field element implementation
func (z *Felt) Add(x, y *Felt) *Felt {
	(*fp.Element)(z).Add((*fp.Element)(x), (*fp.Element)(y))
	return z
}

// Halve forwards the call to underlying field element implementation
func (z *Felt) Halve() {
	(*fp.Element)(z).Halve()
}

// MarshalCBOR lets Felt be encoded in CBOR format with private `val`
func (z *Felt) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal((*fp.Element)(z))
}

// UnmarshalCBOR lets Felt be decoded from CBOR format with private `val`
func (z *Felt) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, (*fp.Element)(z))
}

// Bits forwards the call to underlying field element implementation.
// Provides access to z by returning its value as a little-endian [4]uint64 array.
func (z *Felt) Bits() [4]uint64 {
	return (*fp.Element)(z).Bits()
}

// BigInt forwards the call to underlying field element implementation
func (z *Felt) BigInt(res *big.Int) *big.Int {
	return (*fp.Element)(z).BigInt(res)
}

// Set forwards the call to underlying field element implementation
func (z *Felt) Set(x *Felt) *Felt {
	(*fp.Element)(z).Set((*fp.Element)(x))
	return z
}

// Double forwards the call to underlying field element implementation
func (z *Felt) Double(x *Felt) *Felt {
	(*fp.Element)(z).Double((*fp.Element)(x))
	return z
}

// Sub forwards the call to underlying field element implementation
func (z *Felt) Sub(x, y *Felt) *Felt {
	(*fp.Element)(z).Sub((*fp.Element)(x), (*fp.Element)(y))
	return z
}

func (z *Felt) Neg(x *Felt) *Felt {
	(*fp.Element)(z).Neg((*fp.Element)(x))
	return z
}

// Exp forwards the call to underlying field element implementation
func (z *Felt) Exp(x *Felt, y *big.Int) *Felt {
	(*fp.Element)(z).Exp(fp.Element(*x), y)
	return z
}

// Mul forwards the call to underlying field element implementation
func (z *Felt) Mul(x, y *Felt) *Felt {
	(*fp.Element)(z).Mul((*fp.Element)(x), (*fp.Element)(y))
	return z
}

// Div forwards the call to underlying field element implementation
func (z *Felt) Div(x, y *Felt) *Felt {
	(*fp.Element)(z).Div((*fp.Element)(x), (*fp.Element)(y))
	return z
}

// Cmp forwards the call to underlying field element implementation.
// Returns:
//
//	-1 if z <  x
//	 0 if z == x
//	+1 if z >  x
func (z *Felt) Cmp(x *Felt) int {
	return (*fp.Element)(z).Cmp((*fp.Element)(x))
}

// SetBigInt forwards the call to underlying field element implementation
func (z *Felt) SetBigInt(v *big.Int) *Felt {
	(*fp.Element)(z).SetBigInt(v)
	return z
}

// Uint64 forwards the call to underlying field element implementation
func (z *Felt) Uint64() uint64 {
	return (*fp.Element)(z).Uint64()
}

// TODO: look where this is used, the clone shouldn't return a pointer
func (z *Felt) Clone() *Felt {
	clone := *z
	return &clone
}
