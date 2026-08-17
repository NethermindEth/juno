package felt

import (
	"encoding/binary"
	"math"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/fxamacker/cbor/v2"
)

// Fast, felt-specialized CBOR marshaling.
func (z *Felt) MarshalCBOR() ([]byte, error) {
	data := make([]byte, maxCBORFeltLen)
	n := encodeLimbs(data, z)
	return data[:n], nil
}

// Fast, felt-specialized CBOR unmarshaling.
// Falls back to the generic decoder on shape mismatch
func (z *Felt) UnmarshalCBOR(data []byte) error {
	if decodeFelt(data, z) {
		return nil
	}
	return cbor.Unmarshal(data, (*fp.Element)(z))
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

	// Felts are stored in Montgomery form, so in practice every limb exceeds MaxUint32 and
	// encodes as a full uint64, giving one fixed 37-byte shape. Decode it without the
	// per-limb header switch; anything else falls through to the general loop.
	if len(data) >= maxCBORFeltLen &&
		data[1] == cborUint64AdditionalInfo &&
		data[10] == cborUint64AdditionalInfo &&
		data[19] == cborUint64AdditionalInfo &&
		data[28] == cborUint64AdditionalInfo {
		(*value)[0] = binary.BigEndian.Uint64(data[2:])
		(*value)[1] = binary.BigEndian.Uint64(data[11:])
		(*value)[2] = binary.BigEndian.Uint64(data[20:])
		(*value)[3] = binary.BigEndian.Uint64(data[29:])
		return maxCBORFeltLen, true
	}

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
func decodeFelt(data []byte, value *Felt) bool {
	var felt Felt
	consumed, ok := decodeLimbs(data, &felt)
	if !ok || consumed != len(data) {
		return false
	}

	*value = felt
	return true
}
