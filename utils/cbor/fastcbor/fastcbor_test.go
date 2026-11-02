package fastcbor_test

import (
	"os"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fallbacks counts the reads that fell back to fxamacker. It is not a zap logger, so none is
// sampled away.
type fallbacks struct{ n atomic.Int64 }

func (f *fallbacks) Warn(string, ...zap.Field) { f.n.Add(1) }
func (*fallbacks) Debug(string, ...zap.Field)  {}
func (*fallbacks) Info(string, ...zap.Field)   {}
func (*fallbacks) Error(string, ...zap.Field)  {}
func (*fallbacks) Trace(string, ...zap.Field)  {}

var fell = new(fallbacks)

func TestMain(m *testing.M) {
	fastcbor.SetLogger(fell)
	os.Exit(m.Run())
}

// compare reads data into a T with fast and with fxamacker and requires the same outcome: both
// fail, or both succeed with equal values. It reports whether fast fell back.
func compare[T any](t *testing.T, data []byte) (fellBack bool) {
	t.Helper()
	var want, got T
	wantErr := fxamacker.Unmarshal(data, &want)
	before := fell.n.Load()
	gotErr := fastcbor.Unmarshal(data, &got)
	fellBack = fell.n.Load() != before
	require.Equal(t, wantErr == nil, gotErr == nil, "fxamacker: %v, fast: %v", wantErr, gotErr)
	if wantErr == nil {
		require.Equal(t, want, got)
	}
	return fellBack
}

// readByFast is compare that also requires that fast read the value itself.
func readByFast[T any](t *testing.T, data []byte) {
	t.Helper()
	require.False(t, compare[T](t, data), "fast fell back to fxamacker")
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := fxamacker.Marshal(v)
	require.NoError(t, err)
	return data
}
