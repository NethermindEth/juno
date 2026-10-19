package cbor

import (
	"bytes"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// A decoder reads CBOR into a destination.
type decoder struct {
	unmarshal      func(data []byte, v any) error
	unmarshalFirst func(data []byte, v any) ([]byte, error)
}

// fxamackerDecoder is the default decoder.
// It decodes types with no registered decoder and is the current fallback.
var fxamackerDecoder = decoder{fxamacker.Unmarshal, fxamacker.UnmarshalFirst}

// decoders finds the decoder of a destination by its type, without hashing the interface. A
// destination is looked up by its exact pointer type, *T or **T, so the pointer inside the
// interface is always the one its decoder was written for, and the unsafe casts only erase and
// restore that type. This assumes gc's layout of an interface (the type word followed by the data
// word), which [reflect] also relies on.
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

// A typeDecoder decodes into destinations of one pointer type, *T or **T.
// Registering a decoder for T creates one of each.
type typeDecoder struct {
	name string // destination type
	// decoder is what decoderFor returns for the type, built by bind.
	decoder decoder
	// decodeUnsafe is the registered decoder with the destination as the pointer itself.
	decodeUnsafe func(data []byte, dst unsafe.Pointer) ([]byte, error)
	// reset zeroes dst, undoing a partial read.
	reset func(dst unsafe.Pointer)
	// isZero reports whether dst holds the zero value, which for **T is a nil pointer.
	isZero func(dst unsafe.Pointer) bool
}

// logger reports failed decodes.
var logger atomic.Pointer[log.StructuredLogger]

// SetLogger sets where a decode that falls back to fxamacker is reported. The reports are sampled,
// so a hot read path cannot flood the log. Call it once, at startup.
// todo(danielntmd): It reports errors only, not a wrong value returned without one. The fallback
// and this logger are meant to be removed eventually.
func SetLogger(l log.StructuredLogger) {
	// The first few reports in each interval, then every thereafter'th.
	const (
		interval   = time.Minute
		first      = 5
		thereafter = 100
	)
	l = log.Sampled(l, interval, first, thereafter)
	logger.Store(&l)
}

// unmarshalFirst reads one item into v and returns the bytes after it.
func (d *typeDecoder) unmarshalFirst(data []byte, v any) ([]byte, error) {
	rest, err := d.decodeUnsafe(data, dest(v))
	if err != nil {
		if l := logger.Load(); l != nil {
			(*l).Warn("cbor: decoder failed, decoding with fxamacker",
				zap.String("type", d.name), zap.Error(err))
		}
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
	zeroBytes := make([]byte, typ.Size())
	return bind(&typeDecoder{
		name: reflect.TypeFor[*T]().String(),
		// *T and unsafe.Pointer are passed the same way, so the decoder is called directly.
		decodeUnsafe: *(*func([]byte, unsafe.Pointer) ([]byte, error))(unsafe.Pointer(&decode)),
		reset: func(dst unsafe.Pointer) {
			*(*T)(dst) = *new(T)
		},
		// All-zero bytes are the zero value, but not the reverse (an empty string can still
		// point somewhere), so [reflect] decides the rest.
		isZero: func(dst unsafe.Pointer) bool {
			return bytes.Equal(unsafe.Slice((*byte)(dst), len(zeroBytes)), zeroBytes) ||
				reflect.NewAt(typ, dst).Elem().IsZero()
		},
	})
}

// Head bytes of the two items a pointer treats as nil.
// (RFC 8949, section 3.3)
const (
	simpleNull      = 0xf6
	simpleUndefined = 0xf7
)

// pointerDecoder returns the decoder of **T destinations.
func pointerDecoder[T any](decode func([]byte, *T) ([]byte, error)) *typeDecoder {
	return bind(&typeDecoder{
		name: reflect.TypeFor[**T]().String(),
		decodeUnsafe: func(data []byte, dst unsafe.Pointer) ([]byte, error) {
			// For parity with fxamacker, null and undefined leave the pointer nil.
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
