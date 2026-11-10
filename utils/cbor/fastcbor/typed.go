package fastcbor

import (
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/utils/cbor/cborerr"
)

// DecodeError describes a typed reader failure. The alias preserves errors.As.
type DecodeError = cborerr.DecodeError

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
