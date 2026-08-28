// Package encoder is the node's serialization front door. It names no format.
package encoder

import (
	"io"
	"reflect"
)

// NewEncoder returns a new encoder
func NewEncoder(writer io.Writer) Encoder {
	return engine.NewEncoder(writer)
}

// Marshal returns encoding of param v
func Marshal(v any) ([]byte, error) {
	return engine.Marshal(v)
}

// Unmarshal decodes param v from []byte b
func Unmarshal(b []byte, v any) error {
	return engine.Unmarshal(b, v)
}

// UnmarshalFirst decodes the first data item into param v and returns the remaining bytes
func UnmarshalFirst(b []byte, v any) ([]byte, error) {
	return engine.UnmarshalFirst(b, v)
}

// UnmarshalStrict decodes without the node's type tags and fails on a wire key with no matching
// field. Only projection drift tests need it.
func UnmarshalStrict(b []byte, v any) error {
	return engine.UnmarshalStrict(b, v)
}

// RegisterType registers rType so it carries a tag of its own on the wire.
// It must be called only from init in encoder/registry.
func RegisterType(rType reflect.Type) error {
	return engine.RegisterType(rType)
}

// IsTypeMismatch reports a wire item that does not fit the Go type, as opposed to malformed input.
func IsTypeMismatch(err error) bool {
	return engine.IsTypeMismatch(err)
}
