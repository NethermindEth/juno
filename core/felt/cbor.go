package felt

import (
	"encoding/binary"
	"math"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/fxamacker/cbor/v2"
)

// Fast, felt-specialized CBOR marshaling.
func (z *Felt) MarshalCBOR() ([]byte, error) {
	return encodeFeltLimbs((*fp.Element)(z)), nil
}

// Fast, felt-specialized CBOR unmarshaling.
// Falls back to the generic decoder if the felt is non-canonical or invalid
func (z *Felt) UnmarshalCBOR(data []byte) error {
	if decodeFeltLimbs((*fp.Element)(z), data) {
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

	// cborArrayHeader4 is the first byte of a CBOR array of Limbs items.
	// Top 3 bits are the major type (4 = array), low 5 bits are the length.
	// Type 4 in the top bits is the same as 1<<7, so this byte is
	// 0b100_00100: an array with Limbs (4) elements.
	cborArrayHeader4 = (1 << 7) | Limbs

	// header + 8 bytes following
	maxCBORUintLen = 1 + 8
)

func encodeFeltLimbs(element *fp.Element) []byte {
	// The buffer format is: [cborArrayHeader4] [limb 0] [limb 1] [limb 2] [limb 3]
	buffer := make([]byte, 1, 1+Limbs*maxCBORUintLen)
	buffer[0] = cborArrayHeader4

	for limbIndex := range Limbs {
		buffer = appendCBORUint(buffer, element[limbIndex])
	}

	return buffer
}

func appendCBORUint(buffer []byte, value uint64) []byte {
	switch {
	case value > math.MaxUint32:
		buffer = append(buffer, cborUint64AdditionalInfo)
		return binary.BigEndian.AppendUint64(buffer, value)

	case value > math.MaxUint16:
		buffer = append(buffer, cborUint32AdditionalInfo)
		return binary.BigEndian.AppendUint32(buffer, uint32(value))

	case value > math.MaxUint8:
		buffer = append(buffer, cborUint16AdditionalInfo)
		return binary.BigEndian.AppendUint16(buffer, uint16(value))

	case value >= cborUint8AdditionalInfo:
		return append(buffer, cborUint8AdditionalInfo, byte(value))

	default:
		return append(buffer, byte(value))
	}
}

// Writes element only on success, so a rejected input can't partially
// corrupt it before falling back to the generic decoder.
func decodeFeltLimbs(element *fp.Element, data []byte) bool {
	// The data format is: [cborArrayHeader4] [limb 0] [limb 1] [limb 2] [limb 3]
	if len(data) == 0 || data[0] != cborArrayHeader4 {
		return false
	}

	var limbs fp.Element
	byteOffset := 1

	for limbIndex := range Limbs {
		if byteOffset >= len(data) {
			return false
		}

		headerByte := data[byteOffset]
		byteOffset++

		var limb uint64

		switch {
		case headerByte > cborUint64AdditionalInfo: // invalid header byte for uint64
			return false

		case headerByte == cborUint64AdditionalInfo:
			if byteOffset+8 > len(data) {
				return false
			}
			limb = binary.BigEndian.Uint64(data[byteOffset:])
			byteOffset += 8

		case headerByte == cborUint32AdditionalInfo:
			if byteOffset+4 > len(data) {
				return false
			}
			limb = uint64(binary.BigEndian.Uint32(data[byteOffset:]))
			byteOffset += 4

		case headerByte == cborUint16AdditionalInfo:
			if byteOffset+2 > len(data) {
				return false
			}
			limb = uint64(binary.BigEndian.Uint16(data[byteOffset:]))
			byteOffset += 2

		case headerByte == cborUint8AdditionalInfo:
			if byteOffset+1 > len(data) {
				return false
			}
			limb = uint64(data[byteOffset])
			byteOffset++

		default: // headerByte < cborUint8AdditionalInfo
			limb = uint64(headerByte)
		}

		limbs[limbIndex] = limb
	}

	if byteOffset != len(data) {
		return false
	}

	*element = limbs
	return true
}
