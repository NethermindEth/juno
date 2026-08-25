package cbor_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor"
	fxcbor "github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

func feltSlice(size int) [][4]uint64 {
	slice := make([][4]uint64, size)
	for i := range slice {
		slice[i] = [4]uint64{uint64(i), 1, 2, 3}
	}
	return slice
}

// The fast path recognises one shape; every other shape a felt slice can legally take has to come
// back out the same, which is what the fall back to the general decoder is for.
func TestUnmarshalFeltSliceOtherShapes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []selfDecodingFelt
	}{
		{"null, what a nil slice encodes to", []byte{0xf6}, nil},
		{
			"indefinite-length felt inside an array",
			append([]byte{0x81}, indefiniteLengthFelt...),
			[]selfDecodingFelt{{1, 2, 3, 4}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := []selfDecodingFelt{{9, 9, 9, 9}}
			require.NoError(t, cbor.UnmarshalFeltSlice(test.data, &values))
			require.Equal(t, test.want, values)
		})
	}
}

// 24 crosses into a uint8 length header, 256 into a uint16 one.
var marshalFeltSliceSizes = []int{0, 1, 23, 24, 255, 256}

// fxamacker is the source of truth. It wrote every felt slice that is on disk today.
// A new engine should agree with it.
func TestMarshalFeltSliceMatchesFxamacker(t *testing.T) {
	for _, size := range marshalFeltSliceSizes {
		slice := feltSlice(size)

		fast, err := cbor.MarshalFeltSlice(slice)
		require.NoError(t, err)

		generic, err := fxcbor.Marshal(slice)
		require.NoError(t, err)
		require.Equal(t, generic, fast, "len=%d", size)
	}
}

func TestMarshalFeltSliceMatchesEngine(t *testing.T) {
	for _, size := range marshalFeltSliceSizes {
		slice := feltSlice(size)

		fast, err := cbor.MarshalFeltSlice(slice)
		require.NoError(t, err)

		generic, err := cbor.Marshal(slice)
		require.NoError(t, err)
		require.Equal(t, generic, fast, "len=%d", size)

		var back [][4]uint64
		require.NoError(t, cbor.UnmarshalFeltSlice(fast, &back))
		require.Equal(t, slice, back)
	}
}

// A nil slice marshals as null, not as an empty array, matching the generic encoder.
func TestMarshalFeltSliceNil(t *testing.T) {
	fast, err := cbor.MarshalFeltSlice[selfDecodingFelt](nil)
	require.NoError(t, err)

	generic, err := fxcbor.Marshal([]selfDecodingFelt(nil))
	require.NoError(t, err)
	require.Equal(t, generic, fast)
}
