package fastcbor

import (
	"fmt"
	"reflect"
	"sync"
)

// DecodeError describes a typed reader failure. Formatting is outside the read path.
type DecodeError struct {
	Kind   error
	Detail string
	Extra  int
}

func (e *DecodeError) Error() string {
	if e.Kind == ErrExtraneousData {
		return fmt.Sprintf("%s: %d bytes", e.Kind, e.Extra)
	}
	return e.Kind.Error() + ": " + e.Detail
}

func (e *DecodeError) Unwrap() error { return e.Kind }

type typedReader func([]byte, any) ([]byte, error)

var typedReaders sync.Map // reflect.Type -> typedReader

// RegisterTyped installs a complete typed UnmarshalFirst implementation. Register
// before serving reads. It must enforce the destination and error-cleanup contract
// of UnmarshalFirst. Duplicate registrations and nil readers are programmer errors.
// The routing adapter is outside the typed reader's formal proof boundary.
func RegisterTyped[T any](first func([]byte, *T) ([]byte, error)) {
	if first == nil {
		panic("fastcbor: nil typed reader")
	}
	key := reflect.TypeFor[T]()
	read := typedReader(func(data []byte, dst any) ([]byte, error) { return first(data, dst.(*T)) })
	if _, loaded := typedReaders.LoadOrStore(key, read); loaded {
		panic("fastcbor: duplicate typed reader for " + key.String())
	}
}
