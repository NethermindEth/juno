package cbor

import (
	"encoding/binary"
	"math"
)

const (
	// maxCBORArrayHeaderLen: 1 major/info byte + 4 length bytes (uint32)
	maxCBORArrayHeaderLen = 1 + 4

	// cborNull is the CBOR encoding of null, which the generic encoder emits for a nil slice.
	cborNull = 0xf6
)

// MarshalFeltSlice is the body of felt.Slice's MarshalCBOR.
func MarshalFeltSlice[F FeltLike](slice []F) []byte {
	if slice == nil {
		return []byte{cborNull}
	}

	data := make([]byte, maxCBORArrayHeaderLen+len(slice)*maxCBORFeltLen)

	offset := encodeCBORArrayHeader(data, uint32(len(slice)))
	for idx := range slice {
		offset += encodeLimbs(data[offset:], &slice[idx])
	}

	return data[:offset]
}

// UnmarshalFeltSlice reports whether data is a shape the fast path knows, writing slice only then.
// A caller that gets false has to decode data some other way.
func UnmarshalFeltSlice[F FeltLike](data []byte, slice *[]F) bool {
	size, offset, ok := decodeCBORArrayHeader(data)
	if !ok {
		return false
	}

	// Checking if size was corrupted to avoid allocating a malicious amount of data
	maxPossibleFelts := (len(data) - offset) / minCBORFeltLen
	if size < 0 || size > maxPossibleFelts {
		return false
	}

	buffer := make([]F, size)
	for i := range buffer {
		consumed, ok := decodeLimbs(data[offset:], &buffer[i])
		if !ok {
			return false
		}
		offset += consumed
	}

	if offset != len(data) {
		return false
	}

	*slice = buffer
	return true
}

// encodeCBORArrayHeader writes the CBOR array header for arraySize items into data
// starting at index 0, returning the number of bytes written.
func encodeCBORArrayHeader(data []byte, arraySize uint32) int {
	// Starting with the most common path, which is a small array in this case
	switch {
	case arraySize < cborUint8AdditionalInfo:
		data[0] = cborArrayMajor | byte(arraySize)
		return 1
	case arraySize <= math.MaxUint8:
		data[0] = cborArrayMajor | cborUint8AdditionalInfo
		data[1] = byte(arraySize)
		return 2
	case arraySize <= math.MaxUint16:
		data[0] = cborArrayMajor | cborUint16AdditionalInfo
		binary.BigEndian.PutUint16(data[1:], uint16(arraySize))
		return 3
	default:
		// The larger size should fit into an uint32, bigger arrays do not fit the memory
		data[0] = cborArrayMajor | cborUint32AdditionalInfo
		binary.BigEndian.PutUint32(data[1:], arraySize)
		return maxCBORArrayHeaderLen
	}
}

// decodeCBORArrayHeader reads a CBOR array header at data[0:],
// returning the element count and the number of bytes consumed.
// ok is false when the data is not a CBOR array or its length is encoded larger than a uint32
func decodeCBORArrayHeader(data []byte) (size, offset int, ok bool) {
	if len(data) == 0 {
		return 0, 0, false
	}

	// majorType (first 3 bits) encodes the object type
	majorType := data[0] & 0b1110_0000
	if majorType != cborArrayMajor {
		return 0, 0, false
	}

	// additionalInfo (last 5 bits) encodes the array length or how it follows
	additionalInfo := data[0] & 0b0001_1111

	// Starting with the most common path, which is a small array
	switch {
	case additionalInfo < cborUint8AdditionalInfo:
		return int(additionalInfo), 1, true
	case additionalInfo == cborUint8AdditionalInfo && len(data) >= 2:
		return int(data[1]), 2, true
	case additionalInfo == cborUint16AdditionalInfo && len(data) >= 3:
		return int(binary.BigEndian.Uint16(data[1:])), 3, true
	case additionalInfo == cborUint32AdditionalInfo && len(data) >= 5:
		return int(binary.BigEndian.Uint32(data[1:])), maxCBORArrayHeaderLen, true
	default:
		// A size bigger than an uint32 is not allowed in the fast-path
		return 0, 0, false
	}
}
