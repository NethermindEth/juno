package cbor_test

// TODO(granza): Build a pull of slices to test.
// core/felt/slice_test.go should test them fast vs generic,
// here should test current engine vs canonical encoder (fxmacker).

import (
	"testing"

	"github.com/NethermindEth/juno/encoder/cbor"
	fxcbor "github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

func feltSlice(size int) []limbs {
	slice := make([]limbs, size)
	for i := range slice {
		slice[i] = limbs{uint64(i), 1, 2, 3}
	}
	return slice
}

// 24 crosses into a uint8 length header, 256 into a uint16 one.
var marshalFeltSliceSizes = []int{0, 1, 23, 24, 255, 256}

// fxamacker is the source of truth. It wrote every slice on disk, so engines must agree with it.
// core/slice should not care about this assertion.
func TestMarshalFeltSliceMatchesFxamacker(t *testing.T) {
	for _, size := range marshalFeltSliceSizes {
		slice := feltSlice(size)

		generic, err := fxcbor.Marshal(slice)
		require.NoError(t, err)
		require.Equal(t, generic, cbor.MarshalFeltSlice(slice), "len=%d", size)
	}
}

func TestUnmarshalFeltSliceMatchesFxamacker(t *testing.T) {
	for _, size := range marshalFeltSliceSizes {
		slice := feltSlice(size)

		written, err := fxcbor.Marshal(slice)
		require.NoError(t, err)

		var back []limbs
		require.True(t, cbor.UnmarshalFeltSlice(written, &back), "len=%d", size)
		require.Equal(t, slice, back, "len=%d", size)
	}
}

// A nil slice marshals as null, not as an empty array, matching the library.
func TestMarshalFeltSliceNil(t *testing.T) {
	generic, err := fxcbor.Marshal([]limbs(nil))
	require.NoError(t, err)
	require.Equal(t, generic, cbor.MarshalFeltSlice[limbs](nil))
}
