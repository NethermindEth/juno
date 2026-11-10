// Package fastcbor reads the CBOR records Juno stores into Go values in one pass. It plans a reader
// from each Go type the first time the type is read, and the reader uses a Cursor to read.
//
// The package is a decoder and depends on no other CBOR library. It reads a struct by the wire
// names of its fields, an interface by the tag RegisterType gave its concrete type, and a type that
// decodes itself through UnmarshalCBOR or UnmarshalBinary. What it cannot read, and what it fails
// on, is an error, and the caller reads with another decoder.
//
// A reader never keeps a reference to the input and writes only the destination, which must be the
// zero value. It counts nesting to a fixed limit and never allocates more than the input can back.
//
// The planner uses unsafe and links to the runtime's allocator and map calls (unsafe_New, makemap,
// mapassign, typedmemclr), which the runtime keeps for the packages that link to them
// (go.dev/issue/67401), so a new Go version may need a fallback. A struct read behind a pointer
// shares one allocation with the small structs it points to, so a pointer to one of them keeps the
// whole block alive.
package fastcbor

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/NethermindEth/juno/utils/cbor/cborerr"
)

// Error sentinels are shared with typed readers and must be treated as read-only.
var (
	ErrUnsupported    = cborerr.ErrUnsupported
	ErrExtraneousData = cborerr.ErrExtraneousData
)

// Unmarshal reads data, which holds one item, into dst, which must be the zero value. On an error
// dst holds no partial value, and a dst that was not zero is left as it was.
func Unmarshal[T any](data []byte, dst *T) error {
	rest, err := UnmarshalFirst(data, dst)
	if err == nil && len(rest) != 0 {
		*dst = *new(T)
		return fmt.Errorf("%w: %d bytes", ErrExtraneousData, len(rest))
	}
	return err
}

// UnmarshalFirst reads the first item of data into dst, which must be the zero value, and returns
// the bytes after it. On an error dst holds no partial value, and a dst that was not zero is left
// as it was.
func UnmarshalFirst[T any](data []byte, dst *T) ([]byte, error) {
	if read, ok := typedReaders.Load(reflect.TypeFor[T]()); ok {
		return read.(typedReader)(data, dst)
	}
	if !isZero(dst) {
		// The readers assume a zero value, as other decoders merge into one that is not.
		return nil, fmt.Errorf("%w: the destination is not the zero value", ErrUnsupported)
	}
	read, err := readerFor(reflect.TypeFor[T]())
	if read == nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	rest, err := read(data, unsafe.Pointer(dst))
	if err != nil {
		*dst = *new(T)
		return nil, err
	}
	return rest, nil
}

// isZero reports whether the value dst points to is the zero value. A struct{ A uint64 }{} and an
// empty string are, and a struct{ A uint64 }{9} and a non-nil empty slice are not.
func isZero[T any](dst *T) bool {
	size, base := unsafe.Sizeof(*dst), unsafe.Pointer(dst)
	step := uintptr(1)
	if unsafe.Alignof(*dst) >= 8 {
		step = 8
	}
	zero := true
	for i := uintptr(0); zero && i+step <= size; i += step {
		if step == 8 {
			zero = *(*uint64)(unsafe.Add(base, i)) == 0
		} else {
			zero = *(*byte)(unsafe.Add(base, i)) == 0
		}
	}
	for i := size &^ (step - 1); zero && i < size; i++ {
		zero = *(*byte)(unsafe.Add(base, i)) == 0
	}
	return zero || reflect.ValueOf(dst).Elem().IsZero()
}
