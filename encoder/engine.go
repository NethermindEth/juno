package encoder

import (
	"io"
	"reflect"
)

var engine Engine = newFxamackerEngine()

// Engine is everything a library has to provide to be the node's codec.
type Engine interface {
	NewEncoder(w io.Writer) Encoder
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
	UnmarshalFirst(data []byte, v any) ([]byte, error)
	UnmarshalStrict(data []byte, v any) error
	RegisterType(rType reflect.Type) error
	IsTypeMismatch(err error) bool
}

type Encoder interface {
	Encode(v any) error
}
