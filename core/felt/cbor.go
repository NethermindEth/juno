package felt

import (
	"encoding/binary"
	"math"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"github.com/fxamacker/cbor/v2"
)

// Fast, felt-specialized CBOR marshaling.
func (z *Felt) MarshalCBOR() ([]byte, error) {
	return encodeFeltLimbs(z), nil
}

// Fast, felt-specialized CBOR unmarshaling.
// Falls back to the generic decoder on shape mismatch
func (z *Felt) UnmarshalCBOR(data []byte) error {
	if decodeFeltLimbs(z, data) {
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

func encodeFeltLimbs(value *Felt) []byte {
	// The buffer format is: [cborArrayHeader4] [limb 0] [limb 1] [limb 2] [limb 3]
	buffer := make([]byte, 1+Limbs*maxCBORUintLen)
	buffer[0] = cborArrayHeader4

	index := 1
	for limbIndex := range Limbs {
		limb := value[limbIndex]
		switch {
		case limb > math.MaxUint32:
			buffer[index] = cborUint64AdditionalInfo
			binary.BigEndian.PutUint64(buffer[index+1:], limb)
			index += 1 + 8

		case limb > math.MaxUint16:
			buffer[index] = cborUint32AdditionalInfo
			binary.BigEndian.PutUint32(buffer[index+1:], uint32(limb))
			index += 1 + 4

		case limb > math.MaxUint8:
			buffer[index] = cborUint16AdditionalInfo
			binary.BigEndian.PutUint16(buffer[index+1:], uint16(limb))
			index += 1 + 2

		case limb >= cborUint8AdditionalInfo:
			buffer[index] = cborUint8AdditionalInfo
			buffer[index+1] = byte(limb)
			index += 2

		default:
			buffer[index] = byte(limb)
			index++
		}
	}

	return buffer[:index]
}

// Writes value only on success, so a rejected input can't partially
// corrupt it before falling back to the generic decoder.
func decodeFeltLimbs(value *Felt, data []byte) bool {
	// The data format is: [cborArrayHeader4] [limb 0] [limb 1] [limb 2] [limb 3]
	if len(data) == 0 || data[0] != cborArrayHeader4 {
		return false
	}

	var limbs Felt
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

	*value = limbs
	return true
}
