package felt

import (
	"encoding/binary"
	"math"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/fxamacker/cbor/v2"
)

type Slice[F FeltLike] []F

// maxCBORArrayHeaderLen: 1 major/info byte + 4 length bytes (uint32)
const maxCBORArrayHeaderLen = 1 + 4

func (s Slice[F]) MarshalCBOR() ([]byte, error) {
	if s == nil {
		return []byte{cborlite.Null}, nil
	}

	data := make([]byte, maxCBORArrayHeaderLen+len(s)*maxCBORFeltLen)

	offset := encodeCBORArrayHeader(data, uint32(len(s)))
	for idx := range s {
		offset += encodeLimbs(data[offset:], &s[idx])
	}

	return data[:offset], nil
}

func (s *Slice[F]) UnmarshalCBOR(data []byte) error {
	var slice Slice[F]
	consumed, ok := DecodeSliceCBORPrefix(data, &slice)
	if !ok || consumed != len(data) {
		return s.unmarshalGeneric(data)
	}

	*s = slice
	return nil
}

// DecodeSliceCBORPrefix decodes a felt slice at the start of data, returning how
// many bytes it consumed. Unlike UnmarshalCBOR it allows trailing bytes, so a
// caller decoding a larger structure can keep reading where this left off instead
// of having to walk the array first to find where it ends.
//
// It writes out only on success, and has no generic fallback: the generic decoder
// cannot report how much it consumed, so a caller that hits an unrecognised shape
// has to fall back for the whole structure it is decoding.
func DecodeSliceCBORPrefix[F FeltLike](data []byte, out *Slice[F]) (int, bool) {
	// The header is read once per slice, so the call into cborlite costs nothing
	// measurable here. Reading a limb is a different story: see decodeLimbs, which
	// keeps its own inline switch because it runs once per limb.
	size, offset, ok := cborlite.ArrayHeader(data, 0)
	if !ok || size < 0 {
		return 0, false
	}

	// Checking if size was corrupted to avoid allocating a malicious amount of data
	maxPossibleFelts := (len(data) - offset) / minCBORFeltLen
	if size < 0 || size > maxPossibleFelts {
		return 0, false
	}

	buffer := make([]F, size)
	for i := range buffer {
		consumed, ok := decodeLimbs(data[offset:], &buffer[i])
		if !ok {
			return 0, false
		}
		offset += consumed
	}

	*out = buffer
	return offset, true
}

// unmarshalGeneric handles any shape the fast path does not recognise.
func (s *Slice[F]) unmarshalGeneric(data []byte) error {
	var buffer []F
	if err := cbor.Unmarshal(data, &buffer); err != nil {
		return err
	}

	*s = buffer
	return nil
}

// encodeCBORArrayHeader writes the CBOR array header for arraySize items into data
// starting at index 0, returning the number of bytes written.
func encodeCBORArrayHeader(data []byte, arraySize uint32) int {
	// Starting with the most common path, which is a small array in this case
	switch {
	case arraySize < cborlite.Info1Byte:
		data[0] = cborlite.ArrayMajor | byte(arraySize)
		return 1
	case arraySize <= math.MaxUint8:
		data[0] = cborlite.ArrayMajor | cborlite.Info1Byte
		data[1] = byte(arraySize)
		return 2
	case arraySize <= math.MaxUint16:
		data[0] = cborlite.ArrayMajor | cborlite.Info2Byte
		binary.BigEndian.PutUint16(data[1:], uint16(arraySize))
		return 3
	default:
		// The larger size should fit into an uint32, bigger arrays do not fit the memory
		data[0] = cborlite.ArrayMajor | cborlite.Info4Byte
		binary.BigEndian.PutUint32(data[1:], arraySize)
		return maxCBORArrayHeaderLen
	}
}
