package value

import (
	"errors"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type cborSerializer[V any] struct{}

func (cborSerializer[V]) Marshal(value *V) ([]byte, error) {
	return cbor.Marshal(value)
}

func (cborSerializer[V]) Unmarshal(data []byte, value *V) error {
	return cbor.Unmarshal(data, value)
}

// fastCborSerializer writes with fxamacker and reads with the fastcbor package. A read that
// fastcbor cannot do is reported through the logger and done again with fxamacker.
type fastCborSerializer[V any] struct{}

func (fastCborSerializer[V]) Marshal(value *V) ([]byte, error) {
	return cbor.Marshal(value)
}

func (fastCborSerializer[V]) Unmarshal(data []byte, value *V) error {
	err := fastcbor.Unmarshal(data, value)
	if err == nil || errors.Is(err, fastcbor.ErrExtraneousData) {
		return err
	}
	warnFastCbor[V](err)
	return cbor.Unmarshal(data, value)
}

// UnmarshalFirst reads the first item of data and returns the bytes after it.
func (fastCborSerializer[V]) UnmarshalFirst(data []byte, value *V) ([]byte, error) {
	rest, err := fastcbor.UnmarshalFirst(data, value)
	if err == nil {
		return rest, nil
	}
	warnFastCbor[V](err)
	return cbor.UnmarshalFirst(data, value)
}

var fastCborLogger atomic.Pointer[log.StructuredLogger]

// SetFastCborLogger sets where a read that falls back from fastcbor to fxamacker is reported. The
// reports are sampled, so a hot read path cannot flood the log. Call it once, at startup.
// todo(danielntmd): It reports errors only, not a wrong value returned without one. The fallback
// and this logger are meant to be removed eventually.
func SetFastCborLogger(l log.StructuredLogger) {
	// The first few reports in each interval, then every thereafter'th.
	const (
		interval   = time.Minute
		first      = 5
		thereafter = 100
	)
	l = log.Sampled(l, interval, first, thereafter)
	fastCborLogger.Store(&l)
}

func warnFastCbor[V any](err error) {
	if l := fastCborLogger.Load(); l != nil {
		(*l).Warn("cbor: fastcbor failed, decoding with fxamacker",
			zap.String("type", reflect.TypeFor[V]().String()), zap.Error(err))
	}
}
