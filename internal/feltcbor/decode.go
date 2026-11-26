// Package feltcbor reads a single stored felt from the start of a CBOR item stream.
package feltcbor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// MaxTrimmed is the largest canonical byte string in the compact felt format.
const MaxTrimmed = fp.Bytes - 2

// ErrLegacy requests the caller's general CBOR array decoder.
var ErrLegacy = errors.New("expected an array of four unsigned limbs")

// ErrTruncated indicates an incomplete compact item.
var ErrTruncated = errors.New("truncated byte string")

const (
	// These derive from the CBOR spec, see https://www.rfc-editor.org/rfc/rfc8949.html#section-3
	cborUint8AdditionalInfo  = 24 // 1 byte follows
	cborUint16AdditionalInfo = 25 // 2 bytes follow
	cborUint32AdditionalInfo = 26 // 4 bytes follow
	cborUint64AdditionalInfo = 27 // 8 bytes follow

	// The top 3 bits of a CBOR header byte are the major type, the low 5 the additional info.
	cborMajorTypeMask      = 0b1110_0000
	cborAdditionalInfoMask = 0b0001_1111

	cborUintMajor       = 0 << 5
	cborByteStringMajor = 2 << 5
	cborArrayMajor      = 4 << 5
)

var (
	errTooLong    = errors.New("byte string longer than 32 bytes")
	errOutOfRange = errors.New("value is not below the field modulus")

	errFeltUintTooLarge   = errors.New("uint of 24 or more must be a byte string")
	errFeltSmallBytes     = errors.New("byte string holds a value below 24")
	errFeltLeadingZero    = errors.New("byte string has a leading zero byte")
	errFeltLengthHeader   = errors.New("non-minimal byte string length")
	errFeltLengthEncoding = errors.New("unsupported byte string length encoding")
	errFeltReservedLength = errors.New("31-byte byte string is reserved")
)

// smallMontgomery holds the Montgomery form of every value a one-byte CBOR uint can carry, so
// decoding one is a copy instead of a field multiplication.
var smallMontgomery = func() [cborUint8AdditionalInfo]fp.Element {
	var table [cborUint8AdditionalInfo]fp.Element
	for index := range table {
		table[index].SetUint64(uint64(index))
	}
	return table
}()

// modulusLimbs holds the field modulus as little-endian limbs.
var modulusLimbs = func() [fp.Limbs]uint64 {
	var buffer [fp.Bytes]byte
	fp.Modulus().FillBytes(buffer[:])
	return [fp.Limbs]uint64{
		binary.BigEndian.Uint64(buffer[24:32]),
		binary.BigEndian.Uint64(buffer[16:24]),
		binary.BigEndian.Uint64(buffer[8:16]),
		binary.BigEndian.Uint64(buffer[0:8]),
	}
}()

// rSquare is R squared modulo the field, with R = 2^256. Multiplying a canonical value by it gives
// its Montgomery form.
var rSquare = func() fp.Element {
	var element fp.Element
	element.SetBigInt(new(big.Int).Lsh(big.NewInt(1), 8*fp.Bytes))
	return element
}()

// setTrimmed writes to dst the Montgomery limbs of the value with big-endian bytes body. There are
// at most MaxTrimmed of them, so the value is below the modulus and needs no padded copy or range
// check.
func setTrimmed(element *fp.Element, body []byte) {
	*element = fp.Element{}
	n, i := len(body), 0
	for ; n >= 8; i++ {
		element[i] = binary.BigEndian.Uint64(body[n-8:])
		n -= 8
	}
	if n > 0 {
		var word uint64
		for _, b := range body[:n] {
			word = word<<8 | uint64(b)
		}
		element[i] = word
	}
	element.Mul(element, &rSquare)
}

// decodeByteStringHeader reads the minimal length header of the byte string at data[0:],
// returning the body length and the header size.
func decodeByteStringHeader(data []byte) (length, offset int, err error) {
	additionalInfo := data[0] & cborAdditionalInfoMask
	switch {
	case additionalInfo < cborUint8AdditionalInfo:
		return int(additionalInfo), 1, nil
	case additionalInfo == cborUint8AdditionalInfo && len(data) >= 2:
		if data[1] < cborUint8AdditionalInfo {
			return 0, 0, fmt.Errorf("%w: length %d must be in the header byte", errFeltLengthHeader, data[1])
		}
		return int(data[1]), 2, nil
	case additionalInfo == cborUint8AdditionalInfo:
		return 0, 0, fmt.Errorf("%w: missing the length byte", ErrTruncated)
	default:
		// Wider length headers and indefinite lengths never hold a felt
		return 0, 0, errFeltLengthEncoding
	}
}

// decodeByteStringBody decodes a felt byte string body, telling Montgomery and canonical values
// apart by length.
func decodeByteStringBody(body []byte, value *fp.Element) error {
	length := len(body)
	switch {
	case length == fp.Bytes:
		montgomery := [fp.Limbs]uint64{
			binary.BigEndian.Uint64(body[24:32]),
			binary.BigEndian.Uint64(body[16:24]),
			binary.BigEndian.Uint64(body[8:16]),
			binary.BigEndian.Uint64(body[0:8]),
		}
		if !belowModulus(&montgomery) {
			return errOutOfRange
		}
		*value = fp.Element(montgomery)
		return nil
	case length > MaxTrimmed:
		return errFeltReservedLength
	case length == 0 || (length == 1 && body[0] < cborUint8AdditionalInfo):
		return errFeltSmallBytes
	case body[0] == 0:
		return errFeltLeadingZero
	}

	setTrimmed(value, body)
	return nil
}

// belowModulus reports whether little-endian limbs hold a value below the field modulus.
func belowModulus(limbs *[fp.Limbs]uint64) bool {
	for index := fp.Limbs - 1; index >= 0; index-- {
		if limbs[index] != modulusLimbs[index] {
			return limbs[index] < modulusLimbs[index]
		}
	}
	return false
}

// Decode reads one compact or four-limb legacy felt, leaving dst unchanged on error.
func Decode(data []byte, dst *fp.Element) (int, error) {
	if len(data) == 0 {
		return 0, ErrLegacy
	}
	head := data[0]
	if head < cborUint8AdditionalInfo {
		*dst = smallMontgomery[head]
		return 1, nil
	}
	switch head & cborMajorTypeMask {
	case cborUintMajor:
		return 0, errFeltUintTooLarge
	case cborByteStringMajor:
		return decodeBytes(data, dst)
	}
	if IsWide(data) {
		ReadWide(data, dst)
		return WideLen, nil
	}
	if n := shortLimbs(data, (*[fp.Limbs]uint64)(dst)); n > 0 {
		return n, nil
	}
	return 0, ErrLegacy
}

// decodeBytes reads the canonical trimmed and full-width Montgomery byte strings.
func decodeBytes(data []byte, dst *fp.Element) (int, error) {
	length, offset, err := decodeByteStringHeader(data)
	if err != nil {
		return 0, err
	}
	if length > fp.Bytes {
		return 0, fmt.Errorf("%w: got %d", errTooLong, length)
	}
	if available := len(data) - offset; available < length {
		return 0, fmt.Errorf("%w: header declares %d bytes, got %d", ErrTruncated, length, available)
	}
	if err := decodeByteStringBody(data[offset:offset+length], dst); err != nil {
		return 0, err
	}
	return offset + length, nil
}
