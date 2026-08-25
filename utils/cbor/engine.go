package cbor

import (
	"io"
	"reflect"
)

// engine is an interface to call a CBOR library.
// It mirrors the utils/cbor API.
type engine interface {
	marshal(v any) ([]byte, error)
	unmarshal(data []byte, v any) error
	unmarshalFirst(data []byte, v any) ([]byte, error)
	newEncoder(w io.Writer) Encoder
	registerType(rt reflect.Type) error
	unmarshalStrict(data []byte, v any) error
	isTypeMismatch(err error) bool
}

// Encoder writes CBOR items to a stream.
type Encoder interface {
	Encode(v any) error
}
