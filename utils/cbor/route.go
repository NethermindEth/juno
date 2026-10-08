package cbor

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync/atomic"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
)

// A route is a reader for one Go type, found by the exact type of the destination.
// Only utils/cbor/registry's init adds routes, before concurrent use, so lookups take no lock.
var (
	routes  = map[reflect.Type]*route{}
	enabled atomic.Bool
)

//nolint:gochecknoinits
func init() {
	enabled.Store(os.Getenv("JUNO_CBOR_FAST") != "off")
}

// SetFastEnabled turns routing to the fast decoder on or off, for A/B runs. The environment
// variable JUNO_CBOR_FAST=off sets the initial state.
func SetFastEnabled(on bool) { enabled.Store(on) }

type route struct {
	name string
	// read decodes one item into v, which is a *T or a **T, and leaves it untouched on
	// fast.ErrUnsupported.
	read func(r *fast.Reader, v any) error
	// reset puts v back to the zero value, undoing a partial read.
	reset func(v any)
	// isZero reports whether v points at a zero value (for **T, at a nil pointer).
	isZero func(v any) bool

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
	rt := &route{
		name: typ.String(),
		read: func(r *fast.Reader, v any) error {
			if dst, ok := v.(*T); ok {
				return read(r, dst)
			}
			// **T: null leaves the pointer nil, anything else allocates the value.
			ptr := v.(**T)
			if r.Null() {
				return nil
			}
			value := new(T)
			if err := read(r, value); err != nil {
				return err
			}
			*ptr = value
			return nil
		},
		reset: func(v any) {
			var zero T
			if dst, ok := v.(*T); ok {
				*dst = zero
				return
			}
			*v.(**T) = nil
		},
		isZero: func(v any) bool {
			if dst, ok := v.(*T); ok {
				return reflect.ValueOf(dst).Elem().IsZero()
			}
			return *v.(**T) == nil
		},
	}
	routes[reflect.TypeFor[*T]()] = rt
	routes[reflect.TypeFor[**T]()] = rt
}

// extraneousDataError is what a decode of exactly one item reports about bytes after it.
type extraneousDataError int

func (e extraneousDataError) Error() string {
	return fmt.Sprintf("cbor: %d bytes of extraneous data", int(e))
}

// decode tries the fast decoder for v. It returns handled=false when the caller must
// use fxamacker, with v untouched.
func decode(data []byte, v any) (rest []byte, handled bool, err error) {
	if len(routes) == 0 || !enabled.Load() {
		return nil, false, nil
	}
	rt, ok := routes[reflect.TypeOf(v)]
	if !ok {
		return nil, false, nil
	}
	if !rt.isZero(v) {
		rt.nonZeroTarget.Add(1)
		return nil, false, nil
	}

	reader := fast.NewReader(data)
	if err := rt.read(&reader, v); err != nil {
		rt.reset(v)
		if errors.Is(err, fast.ErrUnsupported) {
			rt.unsupported.Add(1)
			return nil, false, nil
		}
		return nil, true, mapError(rt, err)
	}
	rt.routed.Add(1)
	return reader.Remaining(), true, nil
}

// mapError gives a type mismatch the error type callers already look for.
func mapError(rt *route, err error) error {
	if errors.Is(err, fast.ErrType) {
		return &fxamacker.UnmarshalTypeError{CBORType: "", GoType: rt.name}
	}
	return err
}
