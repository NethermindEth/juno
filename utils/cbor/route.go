package cbor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
)

// A route is a reader for one Go type, found by the exact type of the destination.
// Only utils/cbor/registry's init adds routes, before concurrent use, so lookups take no lock.
var (
	// routes is keyed by the type word of the destination's interface value, which is
	// what reflect.TypeOf returns, without hashing an interface.
	routes  = map[unsafe.Pointer]*route{}
	enabled atomic.Bool
)

//nolint:gochecknoinits
func init() {
	enabled.Store(os.Getenv("JUNO_CBOR_FAST") != "off")
}

// SetFastEnabled turns routing to the fast decoder on or off, for A/B runs. The environment
// variable JUNO_CBOR_FAST=off sets the initial state.
func SetFastEnabled(on bool) { enabled.Store(on) }

// A route serves one destination type, *T or **T, and gets the destination as the
// pointer itself. The routes of *T and **T share their counters.
type route struct {
	// read decodes one item into the destination and leaves it untouched on
	// fast.ErrUnsupported.
	read func(r *fast.Reader, dst unsafe.Pointer) error
	// reset puts the destination back to the zero value, undoing a partial read.
	reset func(dst unsafe.Pointer)
	// isZero reports whether the destination holds the zero value (for **T, a nil pointer).
	isZero func(dst unsafe.Pointer) bool

	*counters
}

type counters struct {
	name          string
	routed        atomic.Uint64
	unsupported   atomic.Uint64
	nonZeroTarget atomic.Uint64
}

// RouteStats counts how a route's reads went.
type RouteStats struct {
	// Routed reads were completed by the fast decoder.
	Routed uint64
	// Unsupported reads met a form the fast decoder leaves to fxamacker, after it had
	// started, and were redone there.
	Unsupported uint64
	// NonZeroTarget reads went to fxamacker without trying, because their destination
	// already held a value and the update semantics are fxamacker's.
	NonZeroTarget uint64
}

// Stats returns the counters of every route, by type name.
func Stats() map[string]RouteStats {
	out := make(map[string]RouteStats, len(routes))
	for _, rt := range routes {
		out[rt.name] = RouteStats{
			Routed:        rt.routed.Load(),
			Unsupported:   rt.unsupported.Load(),
			NonZeroTarget: rt.nonZeroTarget.Load(),
		}
	}
	return out
}

// RegisterReader sends decodes into *T and **T to read. read must write only what it
// decodes, never retain the input, and return fast.ErrUnsupported for a valid form it does not
// read; the wrapper then clears the destination and decodes with fxamacker. Only call this
// from utils/cbor/registry's init, before concurrent use.
func RegisterReader[T any](read func(r *fast.Reader, dst *T) error) {
	typ := reflect.TypeFor[T]()
	shared := &counters{name: typ.String()}
	// A value whose bytes are all zero is the zero value. The converse fails only for
	// values like an empty string that still points somewhere, which reflect decides.
	zeroBytes := make([]byte, typ.Size())
	routes[typeWord(new(T))] = &route{
		read: func(r *fast.Reader, dst unsafe.Pointer) error {
			return read(r, (*T)(dst))
		},
		reset: func(dst unsafe.Pointer) {
			*(*T)(dst) = *new(T)
		},
		isZero: func(dst unsafe.Pointer) bool {
			return bytes.Equal(unsafe.Slice((*byte)(dst), len(zeroBytes)), zeroBytes) ||
				reflect.NewAt(typ, dst).Elem().IsZero()
		},
		counters: shared,
	}
	routes[typeWord(new(*T))] = &route{
		// null leaves the pointer nil, anything else allocates the value.
		read: func(r *fast.Reader, dst unsafe.Pointer) error {
			if r.Null() {
				return nil
			}
			value := new(T)
			if err := read(r, value); err != nil {
				return err
			}
			*(**T)(dst) = value
			return nil
		},
		reset: func(dst unsafe.Pointer) {
			*(**T)(dst) = nil
		},
		isZero: func(dst unsafe.Pointer) bool {
			return *(**T)(dst) == nil
		},
		counters: shared,
	}
}

// eface is the layout of an interface value with no methods.
type eface struct {
	typ, data unsafe.Pointer
}

// typeWord returns the dynamic type of v as the runtime keeps it.
func typeWord(v any) unsafe.Pointer {
	return (*eface)(unsafe.Pointer(&v)).typ
}

// extraneousDataError is what a decode of exactly one item reports about bytes after it.
type extraneousDataError int

func (e extraneousDataError) Error() string {
	return fmt.Sprintf("cbor: %d bytes of extraneous data", int(e))
}

// readers keeps Readers, which would otherwise escape to the heap on every decode.
var readers = sync.Pool{New: func() any { return new(fast.Reader) }}

// decode tries the fast decoder for v. It returns handled=false when the caller must
// use fxamacker, with v untouched.
func decode(data []byte, v any) (rest []byte, handled bool, err error) {
	if len(routes) == 0 || !enabled.Load() {
		return nil, false, nil
	}
	value := (*eface)(unsafe.Pointer(&v))
	rt, ok := routes[value.typ]
	if !ok {
		return nil, false, nil
	}
	// A route's destination is a pointer, which the interface holds as its data word.
	dst := value.data
	if !rt.isZero(dst) {
		rt.nonZeroTarget.Add(1)
		return nil, false, nil
	}

	reader := readers.Get().(*fast.Reader)
	*reader = fast.NewReader(data)
	err = rt.read(reader, dst)
	rest = reader.Remaining()
	*reader = fast.Reader{} // the pool must not hold on to data
	readers.Put(reader)
	if err != nil {
		rt.reset(dst)
		if errors.Is(err, fast.ErrUnsupported) {
			rt.unsupported.Add(1)
			return nil, false, nil
		}
		return nil, true, mapError(rt, err)
	}
	rt.routed.Add(1)
	return rest, true, nil
}

// mapError gives a type mismatch the error type callers already look for.
func mapError(rt *route, err error) error {
	if errors.Is(err, fast.ErrType) {
		return &fxamacker.UnmarshalTypeError{CBORType: "", GoType: rt.name}
	}
	return err
}
