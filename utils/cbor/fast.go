// This wrapper exists only for the fallback to fxamacker. Delete it when the fallback is removed.

package cbor

import (
	"errors"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// UnmarshalFast decodes data into v with fastcbor and, if that cannot, with Unmarshal.
func UnmarshalFast[T any](data []byte, v *T) error {
	err := fastcbor.Unmarshal(data, v)
	if err == nil || errors.Is(err, fastcbor.ErrExtraneousData) {
		return err
	}
	warnFast[T](err)
	return Unmarshal(data, v)
}

// UnmarshalFirstFast decodes the first item of data into v with fastcbor and, if that cannot, with
// UnmarshalFirst. It returns the bytes after the item.
func UnmarshalFirstFast[T any](data []byte, v *T) ([]byte, error) {
	rest, err := fastcbor.UnmarshalFirst(data, v)
	if err == nil {
		return rest, nil
	}
	warnFast[T](err)
	return UnmarshalFirst(data, v)
}

var fastLogger atomic.Pointer[log.StructuredLogger]

// SetFastLogger sets where a decode that falls back from fastcbor is reported, sampled so that a
// hot path cannot flood the log. Call it once, at startup.
// todo(danielntmd): It reports errors only, not a wrong value returned without one.
func SetFastLogger(l log.StructuredLogger) {
	const (
		interval   = time.Minute
		first      = 5
		thereafter = 100
	)
	l = log.Sampled(l, interval, first, thereafter)
	fastLogger.Store(&l)
}

func warnFast[T any](err error) {
	if l := fastLogger.Load(); l != nil {
		(*l).Warn("cbor: fastcbor failed, decoding with fxamacker",
			zap.String("type", reflect.TypeFor[T]().String()), zap.Error(err))
	}
}
