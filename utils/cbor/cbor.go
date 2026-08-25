// Package cbor manages CBOR encoding engines.
// It is the only place where CBOR arithmetic should appear.
package cbor

import (
	"io"
	"reflect"
)

// codec is the engine the node encodes with. This is the swap point.
var codec engine = newFxamackerCodec()

// Marshal returns the CBOR encoding of v, in Juno's stored format.
func Marshal(v any) ([]byte, error) {
	return codec.marshal(v)
}

// Unmarshal decodes one of Juno's stored records into v.
func Unmarshal(data []byte, v any) error {
	return codec.unmarshal(data, v)
}

// UnmarshalFirst decodes the first item of data into v and returns the bytes after it.
func UnmarshalFirst(data []byte, v any) ([]byte, error) {
	return codec.unmarshalFirst(data, v)
}

// NewEncoder returns an encoder writing to w.
func NewEncoder(w io.Writer) Encoder {
	return codec.newEncoder(w)
}

// RegisterType gives a unique CBOR tag to a type.
// It must be called only from init in encoder/registry.
func RegisterType(rType reflect.Type) error {
	return codec.registerType(rType)
}

// UnmarshalStrict decodes without Juno's type tags and fails on a wire key with no matching field.
func UnmarshalStrict(data []byte, v any) error {
	return codec.unmarshalStrict(data, v)
}

// IsTypeMismatch reports whether err comes from a wire item that does not fit the Go type it was
// decoded into, as opposed to malformed input.
func IsTypeMismatch(err error) bool {
	return codec.isTypeMismatch(err)
}
