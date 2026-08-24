package felt

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

const (
	Base16 = 16
	Base10 = 10
)

const (
	Limbs = fp.Limbs // number of 64 bits words needed to represent a Element
	Bits  = fp.Bits  // number of bits needed to represent a Element
	Bytes = fp.Bytes // number of bytes needed to represent a Element

	// Max number of chars to represent a Felt as Hex.
	// 0x at the start + 2 chars per byte.
	MaxFeltAsHexSize = len("0x") + Bytes*2
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

// UnmarshalJSON accepts a quoted, 0x-prefixed hex string and sets z.
//
// TODO(granza): move to UnmarshalText after migrating to json/v2
//
// The UnmarshalJSON interface is faster than the UnmarshalText one on json/v1.
// json/v1 treats the input of UnmarshalText as if it could have escaped chars,
// so it triggers an upfront check scan. This is no longer the case for json/v2.
func (z *Felt) UnmarshalJSON(data []byte) error {
	dataSize := len(data)
	if dataSize < len(`"0x0"`) || data[0] != '"' || data[dataSize-1] != '"' {
		return errors.New("felt: expected a quoted 0x hex string")
	}
	return z.setHex(data[1 : dataSize-1])
}

// setHex parses a 0x-prefixed hex string (without surrounding quotes) into z.
// It is the decode counterpart of AppendText.
func (z *Felt) setHex(data []byte) error {
	if len(data) < len("0x0") || data[0] != '0' || (data[1] != 'x' && data[1] != 'X') {
		return errors.New("felt: expected hex string starting with 0x")
	}
	if len(data) > MaxFeltAsHexSize {
		return errors.New("felt: value exceeds field size")
	}
	digits := data[2:]

	// hex.Decode consumes digits in pairs, so an odd number means a leading zero.
	if len(digits)%2 == 1 {
		var padded [MaxFeltAsHexSize]byte
		padded[0] = '0'
		copiedSize := copy(padded[1:], digits)
		digits = padded[:copiedSize+1]
	}

	var buffer [Bytes]byte
	leadingZeros := Bytes - len(digits)/2
	if _, err := hex.Decode(buffer[leadingZeros:], digits); err != nil {
		return fmt.Errorf("felt: couldn't decode hex value: %w", err)
	}

	return (*fp.Element)(z).SetBytesCanonical(buffer[:])
}

// MarshalText returns the felt as an unquoted 0x hex string with no
// unnecessary leading zeros. The quotes are placed at the encoding/json level.
// Uses a value receiver so encoding/json can call it on non-addressable
// values (struct fields in `any`).
func (z Felt) MarshalText() ([]byte, error) {
	return z.AppendText(make([]byte, 0, MaxFeltAsHexSize))
}

// AppendText appends the felt's 0x hex representation to data.
// The quotes are placed at the encoding/json level.
// It is a json/v2 interface and enforces appending.
func (z Felt) AppendText(data []byte) ([]byte, error) {
	var raw [Bytes]byte
	fp.BigEndian.PutElement(&raw, fp.Element(z))

	// Find the first non zero byte
	firstByteIdx := Bytes - 1
	for chunk := 0; chunk < Bytes; chunk += 8 {
		value := binary.BigEndian.Uint64(raw[chunk:])
		if value != 0 {
			firstByteIdx = chunk + bits.LeadingZeros64(value)/8
			break
		}
	}

	data = append(data, '0', 'x')

	// First byte may need a single hex digit (e.g. 0x3, not 0x03).
	if raw[firstByteIdx] < Base16 {
		data = append(data, "0123456789abcdef"[raw[firstByteIdx]])
		firstByteIdx++
	}

	return hex.AppendEncode(data, raw[firstByteIdx:]), nil
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
