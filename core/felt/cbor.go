package felt

import (
	"encoding/binary"
	"math"

	"github.com/NethermindEth/juno/encoder/cborlite"
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

// DecodeCBORPrefix decodes one felt at the start of data, returning how many bytes
// it consumed. Unlike UnmarshalCBOR it allows trailing bytes, so a caller decoding
// a larger structure can keep reading where this left off instead of having to
// walk the item first to find where it ends.
//
// It writes value only on success, and has no generic fallback: the generic
// decoder cannot report how much it consumed, so a caller that hits an
// unrecognised shape has to fall back for the whole structure it is decoding.
func DecodeCBORPrefix[F FeltLike](data []byte, value *F) (int, bool) {
	return decodeLimbs(data, value)
}

// The major types and additional-info values come from [cborlite], so the felt
// encoding and the readers that consume it cannot drift apart. Only the sizes
// below are felt-specific: a limb is always an unsigned int, so a felt is always
// an array of exactly Limbs of them.
const (
	// cborArrayHeader4 is the first byte of a CBOR array of Limbs items.
	// 0b100_00100: an array with Limbs (4) elements.
	cborArrayHeader4 = cborlite.ArrayMajor | Limbs

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
			data[offset] = cborlite.Info8Byte
			binary.BigEndian.PutUint64(data[offset+1:], limb)
			offset += 1 + 8

		case limb > math.MaxUint16:
			data[offset] = cborlite.Info4Byte
			binary.BigEndian.PutUint32(data[offset+1:], uint32(limb))
			offset += 1 + 4

		case limb > math.MaxUint8:
			data[offset] = cborlite.Info2Byte
			binary.BigEndian.PutUint16(data[offset+1:], uint16(limb))
			offset += 1 + 2

		case limb >= cborlite.Info1Byte:
			data[offset] = cborlite.Info1Byte
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
		case headerByte > cborlite.Info8Byte: // invalid header byte for uint64
			return 0, false

		case headerByte == cborlite.Info8Byte:
			if offset+8 > len(data) {
				return 0, false
			}
			limb = binary.BigEndian.Uint64(data[offset:])
			offset += 8

		case headerByte == cborlite.Info4Byte:
			if offset+4 > len(data) {
				return 0, false
			}
			limb = uint64(binary.BigEndian.Uint32(data[offset:]))
			offset += 4

		case headerByte == cborlite.Info2Byte:
			if offset+2 > len(data) {
				return 0, false
			}
			limb = uint64(binary.BigEndian.Uint16(data[offset:]))
			offset += 2

		case headerByte == cborlite.Info1Byte:
			if offset+1 > len(data) {
				return 0, false
			}
			limb = uint64(data[offset])
			offset++

		default: // headerByte < cborlite.Info1Byte
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
