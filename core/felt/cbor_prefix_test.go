package felt_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The prefix decoders differ from UnmarshalCBOR in three ways, and each one is
// what a caller decoding a larger structure depends on:
//
//   - trailing bytes are allowed, since the item is not the whole buffer
//   - the consumed count is exact, so the caller knows where to read next
//   - there is no generic fallback, so an unrecognised shape is a plain false
//
// A wrong consumed count is the dangerous one: it would not fail here, it would
// silently misread whatever field came after.

func TestDecodeCBORPrefix(t *testing.T) {
	want := felt.FromUint64[felt.Felt](0xdeadbeef)
	encoded, err := want.MarshalCBOR()
	require.NoError(t, err)

	t.Run("reads the felt and reports what it consumed", func(t *testing.T) {
		var got felt.Felt
		consumed, ok := felt.DecodeCBORPrefix(encoded, &got)

		require.True(t, ok)
		assert.Equal(t, len(encoded), consumed)
		assert.Equal(t, want, got)
	})

	t.Run("allows trailing bytes, unlike UnmarshalCBOR", func(t *testing.T) {
		withTrailer := append(append([]byte{}, encoded...), 0xff, 0xff, 0xff)

		var got felt.Felt
		consumed, ok := felt.DecodeCBORPrefix(withTrailer, &got)

		require.True(t, ok)
		assert.Equal(t, len(encoded), consumed, "must stop at the end of the felt")
		assert.Equal(t, want, got)

		// The all-or-nothing entry point rejects the same bytes, which is the whole
		// reason the prefix decoder exists.
		var viaUnmarshal felt.Felt
		assert.Error(t, viaUnmarshal.UnmarshalCBOR(withTrailer))
	})

	t.Run("the consumed count lets a caller chain reads", func(t *testing.T) {
		first := felt.FromUint64[felt.Felt](1)
		second := felt.FromUint64[felt.Felt](0xffffffffffff)

		firstBytes, err := first.MarshalCBOR()
		require.NoError(t, err)
		secondBytes, err := second.MarshalCBOR()
		require.NoError(t, err)
		stream := append(append([]byte{}, firstBytes...), secondBytes...)

		var gotFirst, gotSecond felt.Felt
		consumed, ok := felt.DecodeCBORPrefix(stream, &gotFirst)
		require.True(t, ok)

		next, ok := felt.DecodeCBORPrefix(stream[consumed:], &gotSecond)
		require.True(t, ok)

		assert.Equal(t, first, gotFirst)
		assert.Equal(t, second, gotSecond)
		assert.Equal(t, len(stream), consumed+next)
	})

	t.Run("rejects instead of falling back to the generic decoder", func(t *testing.T) {
		tests := []struct {
			name string
			data []byte
		}{
			{name: "empty", data: []byte{}},
			{name: "truncated", data: encoded[:len(encoded)-1]},
			{name: "not an array", data: []byte{0x18, 0x2a}},
			{name: "array of the wrong length", data: []byte{0x83, 0x01, 0x02, 0x03}},
			{name: "null", data: []byte{0xf6}},
			{name: "a limb that is not an unsigned int", data: []byte{0x84, 0x20, 0x01, 0x02, 0x03}},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				got := felt.FromUint64[felt.Felt](7)
				consumed, ok := felt.DecodeCBORPrefix(test.data, &got)

				assert.False(t, ok)
				assert.Zero(t, consumed)
				assert.Equal(t, felt.FromUint64[felt.Felt](7), got,
					"a rejected input must not touch the destination")
			})
		}
	})
}

func TestDecodeSliceCBORPrefix(t *testing.T) {
	want := felt.Slice[felt.Felt]{
		felt.FromUint64[felt.Felt](1),
		felt.FromUint64[felt.Felt](0xffff),
		felt.FromUint64[felt.Felt](0xffffffffffffffff),
	}
	encoded, err := want.MarshalCBOR()
	require.NoError(t, err)

	t.Run("reads the slice and reports what it consumed", func(t *testing.T) {
		var got felt.Slice[felt.Felt]
		consumed, ok := felt.DecodeSliceCBORPrefix(encoded, &got)

		require.True(t, ok)
		assert.Equal(t, len(encoded), consumed)
		assert.Equal(t, want, got)
	})

	t.Run("allows trailing bytes, unlike UnmarshalCBOR", func(t *testing.T) {
		withTrailer := append(append([]byte{}, encoded...), 0xff, 0xff)

		var got felt.Slice[felt.Felt]
		consumed, ok := felt.DecodeSliceCBORPrefix(withTrailer, &got)

		require.True(t, ok)
		assert.Equal(t, len(encoded), consumed)
		assert.Equal(t, want, got)
	})

	t.Run("empty slice is read as empty, not nil", func(t *testing.T) {
		emptyEncoded, err := felt.Slice[felt.Felt]{}.MarshalCBOR()
		require.NoError(t, err)

		var got felt.Slice[felt.Felt]
		consumed, ok := felt.DecodeSliceCBORPrefix(emptyEncoded, &got)

		require.True(t, ok)
		assert.Equal(t, len(emptyEncoded), consumed)
		assert.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("rejects instead of falling back to the generic decoder", func(t *testing.T) {
		tests := []struct {
			name string
			data []byte
		}{
			{name: "empty", data: []byte{}},
			{name: "truncated", data: encoded[:len(encoded)-1]},
			{name: "not an array", data: []byte{0x18, 0x2a}},
			// MarshalCBOR writes a nil slice as null, so the prefix decoder has to
			// reject it and let the caller decide what an absent slice means.
			{name: "null", data: []byte{0xf6}},
			{name: "an element that is not a felt", data: []byte{0x82, 0x01, 0x02}},
			{name: "count past the remaining bytes", data: []byte{0x98, 0xff, 0x01}},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				got := felt.Slice[felt.Felt]{felt.FromUint64[felt.Felt](9)}
				consumed, ok := felt.DecodeSliceCBORPrefix(test.data, &got)

				assert.False(t, ok)
				assert.Zero(t, consumed)
				assert.Equal(t, felt.Slice[felt.Felt]{felt.FromUint64[felt.Felt](9)}, got,
					"a rejected input must not touch the destination")
			})
		}
	})
}
