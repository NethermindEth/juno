// These tests pin what fastcbor returns for what it does not read, which the callers that read
// again with fxamacker rely on. A read that cannot be done is an error, and the destination is left
// as it was or zero.

package cbor_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/stretchr/testify/require"
)

func TestATypeItCannotReadIsUnsupported(t *testing.T) {
	// An interface that no registered type implements has no reader.
	type record struct {
		X any
		N uint64
	}
	data := marshal(t, record{X: map[string]any{"a": uint64(1)}, N: 3})
	var got record
	require.ErrorIs(t, fastcbor.Unmarshal(data, &got), fastcbor.ErrUnsupported)
	require.Zero(t, got)
	require.True(t, compare[record](t, data))
}

func TestADestinationThatIsNotZeroIsUnsupported(t *testing.T) {
	type record struct {
		A uint64
		B string
		C []uint64
	}
	data := marshal(t, record{A: 1, B: "x", C: []uint64{1, 2}})
	prefilled := record{A: 9, C: []uint64{7, 8, 9, 10}}

	got := prefilled
	require.ErrorIs(t, fastcbor.Unmarshal(data, &got), fastcbor.ErrUnsupported)
	require.Equal(t, prefilled, got, "the destination was written")
}

func TestAFailedReadLeavesTheDestinationZero(t *testing.T) {
	type record struct {
		A uint64
		B []uint64
	}
	data := marshal(t, record{A: 1, B: []uint64{1, 2, 3}})
	var got record
	require.Error(t, fastcbor.Unmarshal(data[:len(data)-1], &got))
	require.Zero(t, got)
}

func TestTrailingBytesAreAnError(t *testing.T) {
	data := append(marshal(t, uint64(5)), 0x01)
	var got uint64
	require.ErrorIs(t, fastcbor.Unmarshal(data, &got), fastcbor.ErrExtraneousData)
	require.Zero(t, got)

	rest, err := fastcbor.UnmarshalFirst(data, &got)
	require.NoError(t, err)
	require.Equal(t, uint64(5), got)
	require.Equal(t, []byte{0x01}, rest)
}
