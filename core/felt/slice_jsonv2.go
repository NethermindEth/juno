//go:build goexperiment.jsonv2

// TODO(granza): Move this to slice.go after jsonv2 is no longer experimental
package felt

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"fmt"
)

func (s Slice[F]) MarshalJSONTo(enc *jsontext.Encoder) error {
	if s == nil {
		return enc.WriteToken(jsontext.Null)
	}

	// account for quotes and commas per felt + '[' and ']' at the ends - the last comma.
	maxSize := len(s)*(MaxFeltAsHexSize+len(`"",`)) + len("[]") - len(",")
	data := make([]byte, 1, maxSize)

	data[0] = '['
	for index := range s {
		if index > 0 {
			data = append(data, ',')
		}
		data = append(data, '"')
		data, _ = Felt(s[index]).AppendText(data)
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
