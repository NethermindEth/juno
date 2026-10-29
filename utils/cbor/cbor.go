// Package cbor provides Juno's CBOR API. Encoding uses fxamacker. Decoding uses the decoder
// registered for the destination's type with [RegisterDecoder].
package cbor

import (
	"io"
	"reflect"

	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
)

// RawMessage is an item that is already CBOR encoded.
type RawMessage = fxamacker.RawMessage

// SelfEncoder is the hook a type implements to write its own encoding.
type SelfEncoder = fxamacker.SelfEncoder

// SelfDecoder is the hook a type implements to read its own encoding.
type SelfDecoder = fxamacker.SelfDecoder

// Encoder writes CBOR values to a stream.
type Encoder = fxamacker.Encoder

// Marshal returns the CBOR encoding of v.
func Marshal(v any) ([]byte, error) {
	return fxamacker.Marshal(v)
}

// Unmarshal decodes a CBOR value from data into v.
func Unmarshal(data []byte, v any) error {
	return decoderFor(v).unmarshal(data, v)
}

// UnmarshalFirst decodes the first CBOR item and returns the remaining bytes.
func UnmarshalFirst(data []byte, v any) ([]byte, error) {
	return decoderFor(v).unmarshalFirst(data, v)
}

// UnmarshalStrict decodes without type tags and rejects unknown fields.
// It is used to check that projections match their stored records.
func UnmarshalStrict(data []byte, v any) error {
	return fxamacker.UnmarshalStrict(data, v)
}

// NewEncoder returns an encoder that writes to w.
func NewEncoder(w io.Writer) Encoder {
	return fxamacker.NewEncoder(w)
}

// RegisterType gives a unique CBOR tag to a type.
// Only call this from utils/cbor/registry's init, before concurrent use.
func RegisterType(t reflect.Type) error {
	return fxamacker.RegisterType(t)
}

// RegisterDecoder makes decode the decoder of *T and **T destinations.
// decode reads one item from the start of data into a dst-owned item and returns the rest.
// If it fails, the item is decoded with fxamacker instead.
// Call it only from init, and decode must be safe for concurrent use.
func RegisterDecoder[T any](decode func(data []byte, dst *T) (rest []byte, err error)) {
	value, pointer := typeWord(new(T)), typeWord(new(*T))
	if decoders[value] != nil || decoders[pointer] != nil {
		panic("cbor: a decoder is already registered for " + reflect.TypeFor[T]().String())
	}
	decoders[value] = valueDecoder(decode)
	decoders[pointer] = pointerDecoder(decode)
}
