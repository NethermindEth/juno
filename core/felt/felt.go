package felt

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json/jsontext"
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

// Typed is a field element told apart at compile time by a phantom kind. Every Typed[K] shares one
// implementation, so all felt types behave and encode the same and converting between them is free.
// Keep its methods inlinable (check with -gcflags=-m=2), typically by delegating to a concrete
// fp.Element helper: the per-kind symbol that method tables point at then calls the helper
// directly instead of bouncing through the dictionary-passing shape body.
type Typed[K any] fp.Element

// Impl returns the underlying field element type
func (z *Typed[K]) Impl() *fp.Element {
	return (*fp.Element)(z)
}

// UnmarshalJSONFrom sets z from a quoted 0x-prefixed hex JSON string; any other JSON value fails.
func (z *Typed[K]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	value, err := dec.ReadValue()
	if err != nil {
		return err
	}
	if value.Kind() != '"' {
		return errors.New("felt: expected a quoted 0x hex string")
	}
	return setHex((*fp.Element)(z), value[1:len(value)-1])
}

// UnmarshalText sets z from an unquoted 0x-prefixed hex string, as map keys and text
// decoders pass it.
func (z *Typed[K]) UnmarshalText(text []byte) error {
	return setHex((*fp.Element)(z), text)
}

// setHex parses a 0x-prefixed hex string (without surrounding quotes) into z.
// It is shared by Felt and Slice JSON decoding.
func setHex(z *fp.Element, data []byte) error {
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

	return z.SetBytesCanonical(buffer[:])
}

// MarshalText returns the felt as an unquoted 0x hex string with no
// unnecessary leading zeros. The quotes are placed at the encoding/json level.
// Uses a value receiver so encoding/json can call it on non-addressable
// values (struct fields in `any`).
func (z Typed[K]) MarshalText() ([]byte, error) {
	return appendHex(make([]byte, 0, MaxFeltAsHexSize), fp.Element(z)), nil
}

// AppendText appends the felt's 0x hex representation to data.
// The quotes are placed at the encoding/json level.
// It is a json/v2 interface and enforces appending.
func (z Typed[K]) AppendText(data []byte) ([]byte, error) {
	return appendHex(data, fp.Element(z)), nil
}

// appendHex appends the 0x hex form of element, without leading zeros, to data.
func appendHex(data []byte, element fp.Element) []byte {
	var raw [Bytes]byte
	fp.BigEndian.PutElement(&raw, element)

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

	return hex.AppendEncode(data, raw[firstByteIdx:])
}

// SetBytes forwards the call to underlying field element implementation
func (z *Typed[K]) SetBytes(e []byte) *Typed[K] {
	(*fp.Element)(z).SetBytes(e)
	return z
}

// SetBytesCanonical forwards the call to underlying field element implementation
func (z *Typed[K]) SetBytesCanonical(e []byte) error {
	return (*fp.Element)(z).SetBytesCanonical(e)
}

// SetString forwards the call to underlying field element implementation
func (z *Typed[K]) SetString(number string) (*Typed[K], error) {
	return z, setString((*fp.Element)(z), number)
}

// setString parses a decimal or hex number into z.
func setString(z *fp.Element, number string) error {
	// get temporary big int from the pool
	vv := bigIntPool.Get().(*big.Int)
	// release object into pool
	defer bigIntPool.Put(vv)

	if _, ok := vv.SetString(number, 0); !ok {
		if _, ok := vv.SetString(number, Base16); !ok {
			return errors.New("can't parse into a big.Int: " + number)
		}
	}

	if vv.BitLen() > fp.Bits {
		return errors.New("can't fit in felt: " + number)
	}

	var bytes [32]byte
	vv.FillBytes(bytes[:])
	return z.SetBytesCanonical(bytes[:])
}

// SetUint64 forwards the call to underlying field element implementation
func (z *Typed[K]) SetUint64(v uint64) *Typed[K] {
	(*fp.Element)(z).SetUint64(v)
	return z
}

// SetRandom forwards the call to underlying field element implementation
func (z *Typed[K]) SetRandom() *Typed[K] {
	setRandom((*fp.Element)(z))
	return z
}

// setRandom fills z from crypto/rand, which is not expected to fail.
func setRandom(z *fp.Element) {
	if _, err := z.SetRandom(); err != nil {
		panic(fmt.Sprintf("unexpected error from rand.Reader: %s", err.Error()))
	}
}

// String forwards the call to underlying field element implementation
func (z *Typed[K]) String() string {
	return "0x" + (*fp.Element)(z).Text(Base16)
}

// ShortString prints the felt to a string in a shortened format
func (z *Typed[K]) ShortString() string {
	return shortString((*fp.Element)(z))
}

// shortString prints the first and last hex digits of z.
func shortString(z *fp.Element) string {
	shortFelt := 8
	hex := z.Text(Base16)

	if len(hex) <= shortFelt {
		return fmt.Sprintf("0x%s", hex)
	}
	return fmt.Sprintf("0x%s...%s", hex[:4], hex[len(hex)-4:])
}

// Text forwards the call to underlying field element implementation
func (z *Typed[K]) Text(base int) string {
	return (*fp.Element)(z).Text(base)
}

// Equal forwards the call to underlying field element implementation
func (z *Typed[K]) Equal(x *Typed[K]) bool {
	return (*fp.Element)(z).Equal((*fp.Element)(x))
}

// Marshal forwards the call to underlying field element implementation
func (z *Typed[K]) Marshal() []byte {
	return (*fp.Element)(z).Marshal()
}

// Unmarshal forwards the call to underlying field element implementation
func (z *Typed[K]) Unmarshal(e []byte) {
	(*fp.Element)(z).Unmarshal(e)
}

// Bytes forwards the call to underlying field element implementation.
// Returns the value of z as a big-endian byte array
func (z *Typed[K]) Bytes() [32]byte {
	return (*fp.Element)(z).Bytes()
}

// IsOne forwards the call to underlying field element implementation
func (z *Typed[K]) IsOne() bool {
	return (*fp.Element)(z).IsOne()
}

// IsZero forwards the call to underlying field element implementation
func (z *Typed[K]) IsZero() bool {
	return (*fp.Element)(z).IsZero()
}

// Add forwards the call to underlying field element implementation
func (z *Typed[K]) Add(x, y *Typed[K]) *Typed[K] {
	(*fp.Element)(z).Add((*fp.Element)(x), (*fp.Element)(y))
	return z
}

// Halve forwards the call to underlying field element implementation
func (z *Typed[K]) Halve() {
	(*fp.Element)(z).Halve()
}

// Bits forwards the call to underlying field element implementation.
// Provides access to z by returning its value as a little-endian [4]uint64 array.
func (z *Typed[K]) Bits() [4]uint64 {
	return (*fp.Element)(z).Bits()
}

// BigInt forwards the call to underlying field element implementation
func (z *Typed[K]) BigInt(res *big.Int) *big.Int {
	return (*fp.Element)(z).BigInt(res)
}

// Set forwards the call to underlying field element implementation
func (z *Typed[K]) Set(x *Typed[K]) *Typed[K] {
	(*fp.Element)(z).Set((*fp.Element)(x))
	return z
}

// Double forwards the call to underlying field element implementation
func (z *Typed[K]) Double(x *Typed[K]) *Typed[K] {
	(*fp.Element)(z).Double((*fp.Element)(x))
	return z
}

// Sub forwards the call to underlying field element implementation
func (z *Typed[K]) Sub(x, y *Typed[K]) *Typed[K] {
	(*fp.Element)(z).Sub((*fp.Element)(x), (*fp.Element)(y))
	return z
}

func (z *Typed[K]) Neg(x *Typed[K]) *Typed[K] {
	(*fp.Element)(z).Neg((*fp.Element)(x))
	return z
}

// Exp forwards the call to underlying field element implementation
func (z *Typed[K]) Exp(x *Typed[K], y *big.Int) *Typed[K] {
	(*fp.Element)(z).Exp(fp.Element(*x), y)
	return z
}

// Mul forwards the call to underlying field element implementation
func (z *Typed[K]) Mul(x, y *Typed[K]) *Typed[K] {
	(*fp.Element)(z).Mul((*fp.Element)(x), (*fp.Element)(y))
	return z
}

// Div forwards the call to underlying field element implementation
func (z *Typed[K]) Div(x, y *Typed[K]) *Typed[K] {
	(*fp.Element)(z).Div((*fp.Element)(x), (*fp.Element)(y))
	return z
}

// Cmp forwards the call to underlying field element implementation.
// Returns:
//
//	-1 if z <  x
//	 0 if z == x
//	+1 if z >  x
func (z *Typed[K]) Cmp(x *Typed[K]) int {
	return (*fp.Element)(z).Cmp((*fp.Element)(x))
}

// SetBigInt forwards the call to underlying field element implementation
func (z *Typed[K]) SetBigInt(v *big.Int) *Typed[K] {
	(*fp.Element)(z).SetBigInt(v)
	return z
}

// Uint64 forwards the call to underlying field element implementation
func (z *Typed[K]) Uint64() uint64 {
	return (*fp.Element)(z).Uint64()
}

// TODO: look where this is used, the clone shouldn't return a pointer
func (z *Typed[K]) Clone() *Typed[K] {
	clone := *z
	return &clone
}
