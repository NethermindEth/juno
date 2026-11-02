// Package fastcbor reads the CBOR records Juno stores into Go values in one pass, with a reader
// that it plans from each type the first time the type is read. It is a decoder only: fxamacker
// still writes every record, in the same format, so reading with it instead is always possible.
//
// It knows no type of Juno but the felts. A reader is built from the shape of the type, as
// fxamacker reads it: the fields of a struct by their wire names, the interfaces that
// cbor.RegisterType gave a tag, and the types that decode themselves. What the planner cannot
// read, and what a reader fails on, is read by fxamacker, and reported through SetLogger.
package fastcbor

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// logger reports the reads that fall back to fxamacker.
var logger atomic.Pointer[log.StructuredLogger]

// SetLogger sets where a read that falls back to fxamacker is reported. The reports are sampled,
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

// Unmarshal reads data, which holds one item, into dst. A T that the planner cannot read, a read
// that fails and a destination that is not the zero value are left to fxamacker.
func Unmarshal[T any](data []byte, dst *T) error {
	rest, err := UnmarshalFirst(data, dst)
	if err == nil && len(rest) != 0 {
		*dst = *new(T)
		return fmt.Errorf("cbor: %d bytes of extraneous data", len(rest))
	}
	return err
}

// UnmarshalFirst reads the first item of data into dst and returns the bytes after it. It falls
// back to fxamacker as Unmarshal does.
func UnmarshalFirst[T any](data []byte, dst *T) ([]byte, error) {
	if !isZero(dst) {
		// The readers assume a zero value, as fxamacker merges into one that is not.
		return fxamacker.UnmarshalFirst(data, dst)
	}
	read, err := readerFor(reflect.TypeFor[T]())
	if read == nil {
		if err != nil {
			warn[T](err)
		}
		return fxamacker.UnmarshalFirst(data, dst)
	}
	rest, err := read(data, unsafe.Pointer(dst))
	if err == nil {
		return rest, nil
	}
	warn[T](err)
	*dst = *new(T)
	return fxamacker.UnmarshalFirst(data, dst)
}

// isZero reports whether the value dst points to is the zero value. All-zero bytes are, and the
// rest is for reflect to say: an empty string can still point somewhere.
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

func warn[T any](err error) {
	if l := logger.Load(); l != nil {
		(*l).Warn("cbor: fast reader failed, decoding with fxamacker",
			zap.String("type", reflect.TypeFor[T]().String()), zap.Error(err))
	}
}
