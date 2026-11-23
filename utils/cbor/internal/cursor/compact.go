package cursor

import (
	"encoding/binary"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// The compact form of a felt, as core/felt writes it. Each value has exactly one encoding, and the
// length alone tells the forms apart.
//
//   - a uint below 24, whose head byte is the value;
//   - a byte string of the value's 1 to 30 significant bytes, big-endian, with the shortest length
//     head and no leading zero, for a value that is not below 24;
//   - a 32-byte string of the four Montgomery limbs, big-endian, for a value of 31 or 32
//     significant bytes.
//
// Other forms return an error.
const (
	// feltBytes is the length of the Montgomery form.
	feltBytes = 32
	// maxTrimmed is the longest trimmed value. A string of 31 bytes is reserved.
	maxTrimmed = feltBytes - 2
)

// smallMontgomery has the Montgomery form of the values 0 to 23, so reading one is a copy.
var smallMontgomery = func() (table [info1Byte][Limbs]uint64) {
	for i := range table {
		var element fp.Element
		element.SetUint64(uint64(i))
		table[i] = element
	}
	return table
}()

// rSquare is R squared modulo the field, with R = 2^256. Multiplying a canonical value by it gives
// its Montgomery form.
var rSquare = func() fp.Element {
	var element fp.Element
	element.SetBigInt(new(big.Int).Lsh(big.NewInt(1), 8*feltBytes))
	return element
}()

// setTrimmed writes to dst the Montgomery limbs of the value with big-endian bytes body. There are
// at most maxTrimmed of them, so the value is below the modulus and needs no padded copy or range
// check.
func setTrimmed(dst *[Limbs]uint64, body []byte) {
	element := (*fp.Element)(dst)
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

// modulus is the field modulus as little-endian limbs.
var modulus = [Limbs]uint64{1, 0, 0, 0x0800000000000011}

// belowModulus reports whether little-endian limbs hold a value below the field modulus.
func belowModulus(limbs *[Limbs]uint64) bool {
	for i := Limbs - 1; i >= 0; i-- {
		if limbs[i] != modulus[i] {
			return limbs[i] < modulus[i]
		}
	}
	return false
}

// rejectTrimmed is the error for a trimmed value that is not in its one form: a byte string of 1 to
// 30 bytes, the first not zero, and not below 24 when it is a single byte.
func (c *Cursor) rejectTrimmed(length int, body []byte) error {
	switch {
	case length > maxTrimmed:
		return c.fail(ErrUnsupported, "byte string of %d bytes is reserved", length)
	case length == 0:
		return c.fail(ErrUnsupported, "empty byte string")
	case length == 1 && body[0] < info1Byte:
		return c.fail(ErrUnsupported, "value below 24 must be a uint")
	default:
		return c.fail(ErrUnsupported, "trimmed value has a leading zero")
	}
}

// compactLimbs reads a compact felt, which starts with a uint or a byte string head. dst is
// written, and the input consumed, only on success. Errors report the start of the felt.
func (c *Cursor) compactLimbs(dst *[Limbs]uint64) error {
	buf := c.buf[c.pos:]
	head := buf[0]
	if head&majorMask == MajorUint {
		if head >= info1Byte {
			return c.fail(ErrUnsupported, "uint head 0x%02x is 24 or more, not a compact felt", head)
		}
		*dst = smallMontgomery[head]
		c.pos++
		return nil
	}

	length, offset := int(head&infoMask), 1
	switch {
	case length < info1Byte:
	case length == info1Byte:
		if len(buf) < 2 {
			return c.fail(ErrTruncated, "input ends inside the length head")
		}
		if buf[1] < info1Byte {
			return c.fail(ErrUnsupported, "length %d is not in its shortest head", buf[1])
		}
		length, offset = int(buf[1]), 2
	default:
		return c.fail(ErrUnsupported, "length head 0x%02x is wider than a byte, or indefinite", head)
	}
	if length > feltBytes {
		return c.fail(ErrUnsupported, "byte string of %d bytes is longer than a felt", length)
	}
	if len(buf)-offset < length {
		return c.fail(ErrTruncated, "byte string of %d bytes, %d left", length, len(buf)-offset)
	}

	body := buf[offset : offset+length]
	switch {
	case length == feltBytes:
		limbs := [Limbs]uint64{
			binary.BigEndian.Uint64(body[24:]),
			binary.BigEndian.Uint64(body[16:]),
			binary.BigEndian.Uint64(body[8:]),
			binary.BigEndian.Uint64(body[:8]),
		}
		if !belowModulus(&limbs) {
			return c.fail(ErrUnsupported, "limbs are not below the field modulus")
		}
		*dst = limbs
	case length > maxTrimmed, length == 0, length == 1 && body[0] < info1Byte, body[0] == 0:
		return c.rejectTrimmed(length, body)
	default:
		setTrimmed(dst, body)
	}
	c.pos += offset + length
	return nil
}
