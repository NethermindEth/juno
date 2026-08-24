package felt

import (
	"bytes"
	"encoding/binary"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"math"

	"github.com/fxamacker/cbor/v2"
)

type Slice[F FeltLike] []F

const (
	// maxCBORArrayHeaderLen: 1 major/info byte + 4 length bytes (uint32)
	maxCBORArrayHeaderLen = 1 + 4

	// cborNull is the CBOR encoding of null, which the generic encoder emits for a nil slice.
	cborNull = 0xf6
)

func (s Slice[F]) MarshalCBOR() ([]byte, error) {
	if s == nil {
		return []byte{cborNull}, nil
	}

	data := make([]byte, maxCBORArrayHeaderLen+len(s)*maxCBORFeltLen)

	offset := encodeCBORArrayHeader(data, uint32(len(s)))
	for idx := range s {
		offset += encodeLimbs(data[offset:], &s[idx])
	}

	return data[:offset], nil
}

func (s *Slice[F]) UnmarshalCBOR(data []byte) error {
	size, offset, ok := decodeCBORArrayHeader(data)
	if !ok {
		return s.unmarshalGeneric(data)
	}

	// Checking if size was corrupted to avoid allocating a malicious amount of data
	maxPossibleFelts := (len(data) - offset) / minCBORFeltLen
	if size < 0 || size > maxPossibleFelts {
		return s.unmarshalGeneric(data)
	}

	buffer := make([]F, size)
	for i := range buffer {
		consumed, ok := decodeLimbs(data[offset:], &buffer[i])
		if !ok {
			return s.unmarshalGeneric(data)
		}
		offset += consumed
	}

	if offset != len(data) {
		return s.unmarshalGeneric(data)
	}

	*s = buffer
	return nil
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

func (s Slice[F]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if s == nil {
		return enc.WriteToken(jsontext.Null)
	}

	// account for quotes and commas per felt + '[' and ']' at the ends - the last comma.
	maxSize := len(s)*(MaxFeltAsHexSize+len(`"",`)) + len("[]") - len(",")
	data := make([]byte, 1, maxSize)

	var err error
	data[0] = '['
	for index := range s {
		if index > 0 {
			data = append(data, ',')
		}
		data = append(data, '"')
		if data, err = Felt(s[index]).AppendText(data); err != nil {
			return err
		}
		data = append(data, '"')
	}
	data = append(data, ']')

	return enc.WriteValue(jsontext.Value(data))
}

// UnmarshalJSONFrom reads a JSON array of 0x-hex strings into the slice.
func (s *Slice[F]) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	kind := dec.PeekKind()
	if kind == 'n' { // null -> nil slice
		_, tokErr := dec.ReadToken()
		if tokErr != nil {
			return tokErr
		}
		*s = nil
		return nil
	}

	if kind != '[' {
		return fmt.Errorf("felt: cannot unmarshal %s into Slice", kind)
	}

	val, valErr := dec.ReadValue()
	if valErr != nil {
		return valErr
	}

	newSlice := (*s)[:0]
	if newSlice == nil {
		newSlice = Slice[F]{}
	}

	for index := 1; index < len(val)-1; {
		switch val[index] {
		case ' ', '\t', '\n', '\r', ',':
			index++
			continue
		case '"':
		default:
			return fmt.Errorf("felt: cannot unmarshal %s element into Slice", kindOf(val[index]))
		}

		relativeEnd := bytes.IndexByte(val[index+1:], '"')
		if relativeEnd < 0 {
			return errors.New("felt: unterminated string in Slice")
		}
		end := index + 1 + relativeEnd

		var newFelt F
		hexErr := asFeltPtr(&newFelt).setHex(val[index+1 : end])
		if hexErr != nil {
			return hexErr
		}

		newSlice = append(newSlice, newFelt)

		index = end + 1
	}

	*s = newSlice
	return nil
}

func kindOf(char byte) jsontext.Kind {
	switch char {
	case 'n', 't', 'f', '"', '{', '[':
		return jsontext.Kind(char)
	default:
		return '0'
	}
}
