package cbor

import (
	"bytes"
	"fmt"
	"reflect"
	"sync/atomic"
	"unsafe"

	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
)

// A decoder reads CBOR into a destination.
type decoder struct {
	unmarshal      func(data []byte, v any) error
	unmarshalFirst func(data []byte, v any) ([]byte, error)
}

// fxamackerDecoder is the default decoder, it decodes types with no registered decoder.
var fxamackerDecoder = decoder{fxamacker.Unmarshal, fxamacker.UnmarshalFirst}

// decoders finds the decoder of a destination by its type, without hashing the interface. A
// destination is looked up by its exact type, *T or **T, so the pointer inside the interface is
// always the one its decoder was written for, and the unsafe casts only erase and restore that
// type. This assumes gc's layout of an interface, the type and then the data word, which reflect
// also relies on.
var decoders = map[unsafe.Pointer]*typeDecoder{}

// decoderFor returns the decoder of v.
// Either the one registered for its type, or fxamacker.
func decoderFor(v any) decoder {
	value := (*eface)(unsafe.Pointer(&v))
	d, ok := decoders[value.typ]
	if !ok || value.data == nil || !d.isZero(value.data) {
		return fxamackerDecoder
	}
	return d.decoder
}

// eface is the layout of an interface value with no methods.
type eface struct {
	typ, data unsafe.Pointer
}

// typeWord returns the dynamic type of v as the runtime keeps it.
func typeWord(v any) unsafe.Pointer {
	return (*eface)(unsafe.Pointer(&v)).typ
}

// dest returns the pointer that v, a pointer to a registered type, holds.
func dest(v any) unsafe.Pointer {
	return (*eface)(unsafe.Pointer(&v)).data
}

// A typeDecoder decodes into destinations of one type, *T or **T, with the decoder registered
// for T.
type typeDecoder struct {
	// decoder is what decoderFor returns for the type, built by bind.
	decoder decoder
	// decodeFirst decodes one item from the start of data into the destination, which it gets as the
	// pointer itself, and returns the bytes after it. It is what UnmarshalFirst does, without
	// clearing the destination on an error.
	decodeFirst func(data []byte, dst unsafe.Pointer) ([]byte, error)
	// reset puts the destination back to the zero value, undoing a partial read.
	reset func(dst unsafe.Pointer)
	// isZero reports whether the destination holds the zero value (for **T, a nil pointer).
	isZero func(dst unsafe.Pointer) bool
}

// decodeErrors counts the decodes a registered decoder failed.
var decodeErrors atomic.Uint64

// DecodeErrors returns how many decodes a registered decoder has failed so far.
func DecodeErrors() uint64 { return decodeErrors.Load() }

// unmarshalFirst reads one item into v and returns the bytes after it.
func (d *typeDecoder) unmarshalFirst(data []byte, v any) ([]byte, error) {
	rest, err := d.decodeFirst(data, dest(v))
	if err != nil {
		decodeErrors.Add(1)
		d.reset(dest(v))
		return fxamackerDecoder.unmarshalFirst(data, v) // fallback
	}
	return rest, nil
}

// unmarshal reads exactly one item into v.
func (d *typeDecoder) unmarshal(data []byte, v any) error {
	rest, err := d.unmarshalFirst(data, v)
	if err == nil && len(rest) != 0 {
		d.reset(dest(v))
		return fmt.Errorf("cbor: %d bytes of extraneous data", len(rest))
	}
	return err
}

// bind builds the decoder that decoderFor returns for d, once, so that a decode allocates nothing
// to call it.
func bind(d *typeDecoder) *typeDecoder {
	d.decoder = decoder{d.unmarshal, d.unmarshalFirst}
	return d
}

// valueDecoder returns the decoder of *T destinations.
func valueDecoder[T any](decode func([]byte, *T) ([]byte, error)) *typeDecoder {
	typ := reflect.TypeFor[T]()
	// A value whose bytes are all zero is the zero value. The converse fails only for
	// values like an empty string that still points somewhere, which reflect decides.
	zeroBytes := make([]byte, typ.Size())
	return bind(&typeDecoder{
		// *T and unsafe.Pointer are passed the same way, so the decoder is called directly,
		// without a closure that would only convert the pointer.
		decodeFirst: *(*func([]byte, unsafe.Pointer) ([]byte, error))(unsafe.Pointer(&decode)),
		reset: func(dst unsafe.Pointer) {
			*(*T)(dst) = *new(T)
		},
		isZero: func(dst unsafe.Pointer) bool {
			return bytes.Equal(unsafe.Slice((*byte)(dst), len(zeroBytes)), zeroBytes) ||
				reflect.NewAt(typ, dst).Elem().IsZero()
		},
	})
}

// Head bytes of the two items a pointer treats as nil.
const (
	simpleNull      = 0xf6
	simpleUndefined = 0xf7
)

// pointerDecoder returns the decoder of **T destinations: a null item leaves the pointer nil, and
// anything else is read into a new T.
func pointerDecoder[T any](decode func([]byte, *T) ([]byte, error)) *typeDecoder {
	return bind(&typeDecoder{
		decodeFirst: func(data []byte, dst unsafe.Pointer) ([]byte, error) {
			if len(data) > 0 && (data[0] == simpleNull || data[0] == simpleUndefined) {
				return data[1:], nil
			}
			value := new(T)
			rest, err := decode(data, value)
			if err != nil {
				return nil, err
			}
			*(**T)(dst) = value
			return rest, nil
		},
		reset: func(dst unsafe.Pointer) {
			*(**T)(dst) = nil
		},
		isZero: func(dst unsafe.Pointer) bool {
			return *(**T)(dst) == nil
		},
	})
}
