//go:build !compactfelt

package felt

import (
	"encoding/binary"
	"math"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

// The default build writes the legacy forms with main's encoders, so it stores what main stores.

// CompactWrites reports whether this build writes felts in the compact forms.
const CompactWrites = false

// maxCBORFeltLen is the longest encoding this build writes.
const maxCBORFeltLen = maxCBORLegacyFeltLen

// Fast, felt-specialized CBOR marshaling.
func (z *Typed[K]) MarshalCBOR() ([]byte, error) {
	data := make([]byte, maxCBORFeltLen)
	n := encodeLimbs(data, z)
	return data[:n], nil
}

// appendCBOR appends the legacy Montgomery-limb encoding of value to data.
func appendCBOR(data []byte, value *fp.Element) []byte {
	var buffer [maxCBORFeltLen]byte
	return append(data, buffer[:encodeLimbs(buffer[:], value)]...)
}

// encodeLimbs puts one CBOR felt to data, returning the number of bytes written.
func encodeLimbs[F FeltLike](data []byte, value *F) int {
	// The format is: [cborLegacyArrayHeader] [limb 0] [limb 1] [limb 2] [limb 3].
	data[0] = cborLegacyArrayHeader

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

// StoredBytes returns the form this build stores a standalone felt value in: the full 32 bytes.
func (z *Typed[K]) StoredBytes() []byte {
	return z.Marshal()
}
