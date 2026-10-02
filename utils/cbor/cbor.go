// Package cbor is Juno's CBOR API. It encodes with fxamacker and decodes with a
// type's registered decoder, falling back to fxamacker.
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

// UnmarshalTypeError identifies a wire item that does not fit the Go type.
type UnmarshalTypeError = fxamacker.UnmarshalTypeError

// Marshal returns the CBOR encoding of v.
func Marshal(v any) ([]byte, error) {
	return fxamacker.Marshal(v)
}

// route pairs a destination type with its registered decoder.
type route struct {
	t      reflect.Type
	decode func([]byte, any) error
}

// routes holds the *T and **T destinations of each registered decoder.
var routes []route

// RegisterDecoder routes reads of t to decode, which must read exactly one CBOR
// value and fail wherever its result would differ from fxamacker's.
// Call it only from utils/cbor/registry's init.
func RegisterDecoder(t reflect.Type, decode func(data []byte, v any) error) {
	// Storage accessors use **T; typed serializers use *T.
	p := reflect.PointerTo(t)
	routes = append(routes, route{p, decode}, route{reflect.PointerTo(p), decode})
}

// Unmarshal decodes data into v. Zero destinations of a registered type use its
// decoder and fall back to fxamacker if it fails; other reads use fxamacker.
func Unmarshal(data []byte, v any) error {
	if decode := registeredDecoder(v); decode != nil {
		if decode(data, v) == nil {
			return nil
		}
		// Discard the partial result before falling back.
		reflect.ValueOf(v).Elem().SetZero()
	}
	return fxamacker.Unmarshal(data, v)
}

func registeredDecoder(v any) func([]byte, any) error {
	t := reflect.TypeOf(v)
	for i := range routes {
		if routes[i].t == t {
			if rv := reflect.ValueOf(v); rv.IsNil() || !rv.Elem().IsZero() {
				return nil
			}
			return routes[i].decode
		}
	}
	return nil
}

// UnmarshalFirst decodes the first CBOR item and returns the remaining bytes.
func UnmarshalFirst(data []byte, v any) ([]byte, error) {
	return fxamacker.UnmarshalFirst(data, v)
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
