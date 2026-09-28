package felt_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/ugorji"
	"github.com/stretchr/testify/require"
)

// The hook reads only definite arrays of four unsigned integers. Other shapes
// must fail without writing the felt.
func TestUgorjiFeltRejectsUnsupportedShapes(t *testing.T) {
	t.Parallel()

	for _, data := range [][]byte{
		{0x80},                            // empty array
		{0x83, 1, 2, 3},                   // short array
		{0x85, 1, 2, 3, 4, 5},             // long array
		{0x9f, 1, 2, 3, 4, 0xff},          // indefinite array
		{0x84, 0x20, 2, 3, 4},             // negative limb
		{0xd8, 100, 0x84, 1, 2, 3, 4},     // tagged array
		{0x84, 0xd8, 100, 1, 2, 3, 4},     // tagged limb
		{0x44, 1, 2, 3, 4},                // byte string
		{0xa2, 1, 2, 3, 4},                // map
		{0x84, 1, 2, 3, 0x1b, 0, 0, 0, 0}, // truncated limb
	} {
		t.Run(hex.EncodeToString(data), func(t *testing.T) {
			t.Parallel()

			value := felt.Felt{9, 8, 7, 6}
			require.Error(t, ugorji.Unmarshal(data, &value))
			require.Equal(t, felt.Felt{9, 8, 7, 6}, value, "a rejected felt is not written")
		})
	}
}

// Ugorji resolves null before calling the hook and zeroes the value.
func TestUgorjiFeltNullZeroes(t *testing.T) {
	t.Parallel()

	for _, data := range [][]byte{{0xf6}, {0xf7}} {
		value := felt.Felt{9, 8, 7, 6}
		require.NoError(t, ugorji.Unmarshal(data, &value))
		require.Equal(t, felt.Felt{}, value)
	}
}

func TestUgorjiFeltAcceptsIntegerEncodings(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		data []byte
		want felt.Felt
	}{
		{[]byte{0x98, 4, 1, 2, 3, 4}, felt.Felt{1, 2, 3, 4}},
		{[]byte{0x84, 0x18, 1, 0x19, 0, 2, 0x1a, 0, 0, 0, 3, 4}, felt.Felt{1, 2, 3, 4}},
		// Null limbs decode as zero.
		{[]byte{0x84, 0xf6, 2, 3, 4}, felt.Felt{0, 2, 3, 4}},
		{[]byte{0x84, 0xf7, 2, 3, 4}, felt.Felt{0, 2, 3, 4}},
		// Ugorji converts integral floats.
		{[]byte{0x84, 0xf9, 0x3c, 0, 2, 3, 4}, felt.Felt{1, 2, 3, 4}},
	} {
		t.Run(hex.EncodeToString(tc.data), func(t *testing.T) {
			t.Parallel()

			var value felt.Felt
			require.NoError(t, ugorji.Unmarshal(tc.data, &value))
			require.Equal(t, tc.want, value)
		})
	}
}

func TestUgorjiFeltBoundaries(t *testing.T) {
	t.Parallel()

	for _, limb := range []uint64{
		0, 1, 23, 24, 255, 256, 65535, 65536, 1<<32 - 1, 1 << 32, 1<<64 - 1,
	} {
		value := felt.Felt{limb, limb, limb, limb}
		data, err := value.MarshalCBOR()
		require.NoError(t, err)
		t.Run(hex.EncodeToString(data), func(t *testing.T) {
			t.Parallel()

			input := bytes.Clone(data)
			var actual felt.Felt
			require.NoError(t, ugorji.Unmarshal(input, &actual))
			clear(input)
			require.Equal(t, value, actual)
			for n := range data {
				require.Error(t, ugorji.Unmarshal(data[:n], &actual))
			}
		})
	}
}

// Where both accept an input, the hook decodes the same felt as UnmarshalCBOR.
func FuzzUgorjiFeltMatchesUnmarshalCBOR(f *testing.F) {
	for _, value := range []felt.Felt{{}, {1, 2, 3, 4}, {1<<64 - 1, 1 << 32, 65535, 23}} {
		data, err := value.MarshalCBOR()
		require.NoError(f, err)
		f.Add(data)
	}
	for _, tc := range decodeCornerCases {
		f.Add(tc.data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var fromUgorji, fromFelt felt.Felt
		errUgorji := ugorji.Unmarshal(data, &fromUgorji)
		if errUgorji == nil && fromFelt.UnmarshalCBOR(data) == nil {
			require.Equal(t, fromFelt, fromUgorji)
		}
	})
}
