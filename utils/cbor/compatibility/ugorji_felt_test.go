package cbor_test

import (
	"bytes"
	"encoding/hex"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/ugorji"
	"github.com/stretchr/testify/require"
)

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

// Where both accept an input, Ugorji decodes the same felt as UnmarshalCBOR.
func FuzzUgorjiFeltMatchesUnmarshalCBOR(f *testing.F) {
	for _, value := range []felt.Felt{{}, {1, 2, 3, 4}, {1<<64 - 1, 1 << 32, 65535, 23}} {
		data, err := value.MarshalCBOR()
		require.NoError(f, err)
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if mayHoldBignums(data) {
			return
		}
		var fromUgorji, fromFelt felt.Felt
		errUgorji := ugorji.Unmarshal(data, &fromUgorji)
		if errUgorji == nil && fromFelt.UnmarshalCBOR(data) == nil {
			require.Equal(t, fromFelt, fromUgorji)
		}
	})
}

func TestUgorjiSlice(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		data []byte
		want felt.Slice[felt.Felt]
	}{
		{name: "null", data: []byte{0xf6}},
		{name: "empty", data: []byte{0x80}, want: felt.Slice[felt.Felt]{}},
		{
			name: "two felts",
			data: []byte{0x82, 0x84, 0, 0, 0, 0, 0x84, 1, 2, 3, 4},
			want: felt.Slice[felt.Felt]{{}, {1, 2, 3, 4}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := felt.Slice[felt.Felt]{{9, 8, 7, 6}}
			require.NoError(t, ugorji.Unmarshal(tc.data, &value))
			require.Equal(t, tc.want, value)
		})
	}

	t.Run("large slice", func(t *testing.T) {
		t.Parallel()

		expected := make(felt.Slice[felt.Felt], 5000)
		for i := range expected {
			expected[i] = felt.Felt{uint64(i), 1, 2, 3}
		}
		data, err := expected.MarshalCBOR()
		require.NoError(t, err)

		input := bytes.Clone(data)
		var actual felt.Slice[felt.Felt]
		require.NoError(t, ugorji.Unmarshal(input, &actual))
		clear(input)
		require.Equal(t, expected, actual)
		for _, n := range []int{1, 3, len(data) / 2, len(data) - 1} {
			require.Error(t, ugorji.Unmarshal(data[:n], &actual))
		}
	})
}

// Where both accept an input, Ugorji decodes the same slice as UnmarshalCBOR.
func FuzzUgorjiSliceMatchesUnmarshalCBOR(f *testing.F) {
	for _, value := range []felt.Slice[felt.Felt]{
		nil, {}, {{1, 2, 3, 4}}, {{}, {1<<64 - 1, 1 << 32, 65535, 23}},
	} {
		data, err := value.MarshalCBOR()
		require.NoError(f, err)
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if mayHoldBignums(data) {
			return
		}
		var fromUgorji, fromSlice felt.Slice[felt.Felt]
		errUgorji := ugorji.Unmarshal(data, &fromUgorji)
		if errUgorji == nil && fromSlice.UnmarshalCBOR(data) == nil {
			require.Equal(t, fromSlice, fromUgorji)
		}
	})
}

// mayHoldBignums flags potential tags 2-5, which Ugorji can misdecode into
// ordinary integers. Fuzzers skip them; Juno never writes those fields this way.
func mayHoldBignums(data []byte) bool {
	return slices.ContainsFunc(data, func(b byte) bool { return b >= 0xc2 && b <= 0xc5 })
}
