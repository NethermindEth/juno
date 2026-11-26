package feltcbor

import (
	"encoding/binary"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

const (
	legacyHead = cborArrayMajor | fp.Limbs
	// WideLen is the encoded size of a legacy felt with four full-width limbs.
	WideLen = 1 + fp.Limbs*9
)

// IsWide reports whether buf starts with a felt whose four limbs all take the eight-byte form.
func IsWide(buf []byte) bool {
	return len(buf) >= WideLen && buf[0] == legacyHead &&
		buf[1] == cborUint64AdditionalInfo && buf[10] == cborUint64AdditionalInfo &&
		buf[19] == cborUint64AdditionalInfo && buf[28] == cborUint64AdditionalInfo
}

// ReadWide decodes the fixed-width legacy form after IsWide succeeds.
func ReadWide(buf []byte, dst *fp.Element) {
	dst[0] = binary.BigEndian.Uint64(buf[2:])
	dst[1] = binary.BigEndian.Uint64(buf[11:])
	dst[2] = binary.BigEndian.Uint64(buf[20:])
	dst[3] = binary.BigEndian.Uint64(buf[29:])
}

// shortLimbs reads a felt whose limbs have any width and returns the bytes it took, or 0 for
// another form.
func shortLimbs(buf []byte, dst *[fp.Limbs]uint64) int {
	if len(buf) == 0 || buf[0] != legacyHead {
		return 0
	}
	var limbs [fp.Limbs]uint64
	n := 1
	for i := range limbs {
		if n >= len(buf) {
			return 0
		}
		head := buf[n]
		switch {
		case head < cborUint8AdditionalInfo:
			limbs[i] = uint64(head)
			n++
		case head == cborUint8AdditionalInfo && n+2 <= len(buf):
			limbs[i] = uint64(buf[n+1])
			n += 2
		case head == cborUint16AdditionalInfo && n+3 <= len(buf):
			limbs[i] = uint64(binary.BigEndian.Uint16(buf[n+1:]))
			n += 3
		case head == cborUint32AdditionalInfo && n+5 <= len(buf):
			limbs[i] = uint64(binary.BigEndian.Uint32(buf[n+1:]))
			n += 5
		case head == cborUint64AdditionalInfo && n+9 <= len(buf):
			limbs[i] = binary.BigEndian.Uint64(buf[n+1:])
			n += 9
		default:
			return 0
		}
	}
	*dst = limbs
	return n
}
