package cbor

import (
	"encoding/binary"
	"math"
)

// The number of 64 bit words a felt is made of.
const Limbs = 4

// FeltLike mirrors felt.FeltLike.
type FeltLike interface {
	~[Limbs]uint64
}

func MarshalFelt[F FeltLike](value *F) []byte {
	data := make([]byte, maxCBORFeltLen)
	n := encodeLimbs(data, value)
	return data[:n]
}

// UnmarshalFelt reports whether data is a shape the fast path knows, writing value only then.
// A caller that gets false has to decode data some other way.
func UnmarshalFelt[F FeltLike](data []byte, value *F) bool {
	return decodeFelt(data, value)
}

const (
	// These derive from the CBOR spec
	// Limb types are always unsigned int
	// The following numbers represent the unsigned integer size
	// See: https://www.rfc-editor.org/rfc/rfc8949.html#section-3
	cborUint8AdditionalInfo  = 24 // 1 byte follows
	cborUint16AdditionalInfo = 25 // 2 bytes follow
	cborUint32AdditionalInfo = 26 // 4 bytes follow
	cborUint64AdditionalInfo = 27 // 8 bytes follow

	// cborArrayMajor the major type that represents an Array
	// Top 3 bits are the major type (4 = array), low 5 bits are the length.
	cborArrayMajor = 4 << 5

	// cborArrayHeader4 is the first byte of a CBOR array of Limbs items.
	// 0b100_00100: an array with Limbs (4) elements.
	cborArrayHeader4 = cborArrayMajor | Limbs

	// Header + 8 bytes following
	maxCBORUintLen = 1 + 8
	// Header + Limbs * maxCBORUintLen
	maxCBORFeltLen = 1 + Limbs*maxCBORUintLen
	// Header + Limbs
	minCBORFeltLen = 1 + Limbs
)

// encodeLimbs puts one CBOR felt to data, returning the number of bytes written.
func encodeLimbs[F FeltLike](data []byte, value *F) int {
	// The format is: [cborArrayHeader4] [limb 0] [limb 1] [limb 2] [limb 3].
	data[0] = cborArrayHeader4

	offset := 1
	for limbIndex := range Limbs {
		limb := (*value)[limbIndex]

		// Starting with the most common path, which is a large limb
		switch {
		case limb > math.MaxUint32:
			data[offset] = cborUint64AdditionalInfo
			binary.BigEndian.PutUint64(data[offset+1:], limb)
			offset += 1 + 8

		case limb > math.MaxUint16:
			data[offset] = cborUint32AdditionalInfo
			binary.BigEndian.PutUint32(data[offset+1:], uint32(limb))
			offset += 1 + 4

		case limb > math.MaxUint8:
			data[offset] = cborUint16AdditionalInfo
			binary.BigEndian.PutUint16(data[offset+1:], uint16(limb))
			offset += 1 + 2

		case limb >= cborUint8AdditionalInfo:
			data[offset] = cborUint8AdditionalInfo
			data[offset+1] = byte(limb)
			offset += 2

		default:
			data[offset] = byte(limb)
			offset++
		}
	}

	return offset
}

// decodeLimbs decodes one limb-encoded felt at data[0:],
// returning the number of bytes consumed and a flag to signal succes.
// It writes value only on success, so a rejected input can't partially
// corrupt it before falling back to the generic decoder.
func decodeLimbs[F FeltLike](data []byte, value *F) (int, bool) {
	// The data format is: [cborArrayHeader4] [limb 0] [limb 1] [limb 2] [limb 3]
	if len(data) == 0 || data[0] != cborArrayHeader4 {
		return 0, false
	}

	// Header-byte offset of each limb in the all-uint64 shape; the 8-byte payload follows.
	const (
		limb0Header = 1
		limb1Header = limb0Header + maxCBORUintLen
		limb2Header = limb1Header + maxCBORUintLen
		limb3Header = limb2Header + maxCBORUintLen
	)

	// Felts are stored in Montgomery form, so in practice every limb exceeds MaxUint32 and
	// encodes as a full uint64, giving one fixed 37-byte shape. Decode it without the
	// per-limb header switch; anything else falls through to the general loop.
	if len(data) >= maxCBORFeltLen &&
		data[limb0Header] == cborUint64AdditionalInfo &&
		data[limb1Header] == cborUint64AdditionalInfo &&
		data[limb2Header] == cborUint64AdditionalInfo &&
		data[limb3Header] == cborUint64AdditionalInfo {
		(*value)[0] = binary.BigEndian.Uint64(data[limb0Header+1 : limb1Header])
		(*value)[1] = binary.BigEndian.Uint64(data[limb1Header+1 : limb2Header])
		(*value)[2] = binary.BigEndian.Uint64(data[limb2Header+1 : limb3Header])
		(*value)[3] = binary.BigEndian.Uint64(data[limb3Header+1 : maxCBORFeltLen])
		return maxCBORFeltLen, true
	}

	return decodeVariableLimbs(data, value)
}

// decodeVariableLimbs decodes the limbs at data[1:] one header byte at a time, covering the shapes
// the fixed-shape fast path rejects; data[0] must already be a validated array header.
func decodeVariableLimbs[F FeltLike](data []byte, value *F) (int, bool) {
	var limbs F
	offset := 1

	for limbIndex := range Limbs {
		if offset >= len(data) {
			return 0, false
		}

		headerByte := data[offset]
		offset++

		var limb uint64
		switch {
		case headerByte > cborUint64AdditionalInfo: // invalid header byte for uint64
			return 0, false

		case headerByte == cborUint64AdditionalInfo:
			if offset+8 > len(data) {
				return 0, false
			}
			limb = binary.BigEndian.Uint64(data[offset:])
			offset += 8

		case headerByte == cborUint32AdditionalInfo:
			if offset+4 > len(data) {
				return 0, false
			}
			limb = uint64(binary.BigEndian.Uint32(data[offset:]))
			offset += 4

		case headerByte == cborUint16AdditionalInfo:
			if offset+2 > len(data) {
				return 0, false
			}
			limb = uint64(binary.BigEndian.Uint16(data[offset:]))
			offset += 2

		case headerByte == cborUint8AdditionalInfo:
			if offset+1 > len(data) {
				return 0, false
			}
			limb = uint64(data[offset])
			offset++

		default: // headerByte < cborUint8AdditionalInfo
			limb = uint64(headerByte)
		}

		limbs[limbIndex] = limb
	}

	*value = limbs
	return offset, true
}

// decodeFelt decodes a single felt that must span all of data, rejecting trailing bytes.
// It returns a flag to signal success, and writes value only on success.
func decodeFelt[F FeltLike](data []byte, value *F) bool {
	var felt F
	consumed, ok := decodeLimbs(data, &felt)
	if !ok || consumed != len(data) {
		return false
	}

	*value = felt
	return true
}
