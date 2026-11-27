// These tests pin the decode with a fallback of utils/cbor. A read that fastcbor cannot do is
// reported and done again by fxamacker, which gives the value fxamacker alone would give.

package cbor_test

import (
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// warnings counts the warnings it is given.
type warnings struct{ n atomic.Int64 }

func (w *warnings) Warn(string, ...zap.Field) { w.n.Add(1) }
func (*warnings) Debug(string, ...zap.Field)  {}
func (*warnings) Info(string, ...zap.Field)   {}
func (*warnings) Error(string, ...zap.Field)  {}
func (*warnings) Trace(string, ...zap.Field)  {}

type plainRecord struct{ A uint64 }

// unreadable has a field of an interface type that nothing registered implements, which only
// fxamacker reads.
type unreadable struct {
	N uint64
	V any
}

func TestUnmarshalFast(t *testing.T) {
	reports := new(warnings)
	cbor.ConfigureFastReads(false, reports)

	t.Run("disabled reads use fxamacker directly", func(t *testing.T) {
		want := unreadable{N: 3, V: "a"}
		data := marshal(t, want)
		var got unreadable
		require.NoError(t, cbor.UnmarshalFast(data, &got))
		require.Equal(t, want, got)

		var first unreadable
		rest, err := cbor.UnmarshalFirstFast(append(data, 0x07), &first)
		require.NoError(t, err)
		require.Equal(t, want, first)
		require.Equal(t, []byte{0x07}, rest)
		require.Zero(t, reports.n.Load())
	})

	cbor.ConfigureFastReads(true, reports)
	t.Cleanup(func() { cbor.ConfigureFastReads(false, nil) })

	t.Run("a value fastcbor reads is not reported", func(t *testing.T) {
		data := marshal(t, plainRecord{A: 1})
		var got plainRecord
		require.NoError(t, cbor.UnmarshalFast(data, &got))
		require.Equal(t, plainRecord{A: 1}, got)
		require.Zero(t, reports.n.Load())
	})

	t.Run("a value it cannot read is reported and read by fxamacker", func(t *testing.T) {
		want := unreadable{N: 3, V: []any{uint64(1), "a"}}
		var got unreadable
		require.NoError(t, cbor.UnmarshalFast(marshal(t, want), &got))
		require.Equal(t, want, got)
		require.Equal(t, int64(1), reports.n.Load())
	})

	t.Run("bytes after the value are an error that is not reported", func(t *testing.T) {
		before := reports.n.Load()
		var got uint64
		err := cbor.UnmarshalFast(append(marshal(t, uint64(5)), 0x01), &got)
		require.ErrorContains(t, err, "extraneous data")
		require.Equal(t, before, reports.n.Load())
	})
}

func TestUnmarshalFirstFast(t *testing.T) {
	reports := new(warnings)
	cbor.ConfigureFastReads(true, reports)
	t.Cleanup(func() { cbor.ConfigureFastReads(false, nil) })

	var got plainRecord
	rest, err := cbor.UnmarshalFirstFast(append(marshal(t, plainRecord{A: 5}), 0x07), &got)
	require.NoError(t, err)
	require.Equal(t, uint64(5), got.A)
	require.Equal(t, []byte{0x07}, rest)

	var other unreadable
	rest, err = cbor.UnmarshalFirstFast(append(marshal(t, unreadable{N: 1, V: "x"}), 0x07), &other)
	require.NoError(t, err)
	require.Equal(t, unreadable{N: 1, V: "x"}, other)
	require.Equal(t, int64(1), reports.n.Load())
	require.Equal(t, []byte{0x07}, rest)
}
