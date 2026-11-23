// This wrapper exists only for the fallback to fxamacker. Delete it when the fallback is removed
// and fastcbor is unconditional.

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
	logger := fastLogger.Load()
	if logger == nil {
		return Unmarshal(data, v)
	}
	err := fastcbor.Unmarshal(data, v)
	if err == nil || errors.Is(err, fastcbor.ErrExtraneousData) {
		return err
	}
	warnFast[T](*logger, err)
	return Unmarshal(data, v)
}

// UnmarshalFirstFast decodes the first item of data into v with fastcbor and, if that cannot, with
// UnmarshalFirst. It returns the bytes after the item.
func UnmarshalFirstFast[T any](data []byte, v *T) ([]byte, error) {
	logger := fastLogger.Load()
	if logger == nil {
		return UnmarshalFirst(data, v)
	}
	rest, err := fastcbor.UnmarshalFirst(data, v)
	if err == nil {
		return rest, nil
	}
	warnFast[T](*logger, err)
	return UnmarshalFirst(data, v)
}

var fastLogger atomic.Pointer[log.StructuredLogger]

// ConfigureFastReads sets the process-wide decoder mode at startup. When disabled,
// UnmarshalFast and UnmarshalFirstFast use fxamacker directly. When enabled,
// the supplied logger reports sampled fallbacks.
// todo(danielntmd): It reports errors only, not a wrong value returned without one.
func ConfigureFastReads(enabled bool, l log.StructuredLogger) {
	if !enabled {
		fastLogger.Store(nil)
		return
	}

	const (
		interval   = time.Minute
		first      = 5
		thereafter = 100
	)
	l = log.Sampled(l, interval, first, thereafter)
	fastLogger.Store(&l)
}

func warnFast[T any](l log.StructuredLogger, err error) {
	if l != nil {
		l.Warn("cbor: fastcbor failed, decoding with fxamacker",
			zap.String("type", reflect.TypeFor[T]().String()), zap.Error(err))
	}
}
