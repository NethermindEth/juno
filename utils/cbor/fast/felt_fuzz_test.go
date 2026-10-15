package fast_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

func feltSeeds() [][]byte {
	full := []byte{
		0x84, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 0xff, 0, 0, 0, 0, 0, 0, 1,
		0x1b, 0, 0, 0, 0, 0, 0, 0, 9, 0x1b, 0x07, 0xff, 0xff, 0xff, 0xff, 0xff, 0xf1, 0x30,
	}
	return [][]byte{
		full,
		{0x84, 0x00, 0x18, 0x18, 0x19, 0x01, 0x00, 0x1a, 0, 1, 0, 0},
		{0x80},
		{0x82, 0x01, 0x02},
		{0x85, 1, 2, 3, 4, 5},
		{0x84, 1, 2, 3, 0x20},
		{0x81, 0x84, 1, 2, 3, 4},
		{0xf6},
		{0x9f, 0xff},
		append([]byte{0x82}, append(append([]byte(nil), full...), full...)...),
	}
}

// FuzzFeltsMatchFxamacker checks the felt readers against fxamacker decoding into the same
// shapes, a [4]uint64 and a slice of them. A reader that succeeds must agree with fxamacker on
// the value and the remainder. Where fxamacker succeeds, a reader may only fail by being
// stricter about types (ErrType) or by leaving a form to the fallback (ErrUnsupported).
func FuzzFeltsMatchFxamacker(f *testing.F) {
	for _, s := range append(feltSeeds(), seeds()...) {
		f.Add(s)
	}
	allowed := func(err error) bool {
		return errors.Is(err, fast.ErrType) || errors.Is(err, fast.ErrUnsupported)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		t.Run("limbs", func(t *testing.T) {
			var want, got [fast.Limbs]uint64
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			r := fast.NewReader(data)
			err := r.Limbs(&got)
			if err != nil {
				require.Zero(t, r.Pos(), "a failed read moved the cursor")
				require.True(t, fxErr != nil || allowed(err), "fxamacker accepts what Limbs rejects: %v", err)
				return
			}
			require.NoError(t, fxErr)
			require.Equal(t, want, got)
			require.Equal(t, rest, r.Remaining())
		})
		t.Run("slice", func(t *testing.T) {
			var want [][fast.Limbs]uint64
			rest, fxErr := fxamacker.UnmarshalFirst(data, &want)
			r := fast.NewReader(data)
			got, err := fast.LimbSlice[[fast.Limbs]uint64](&r)
			if err != nil {
				require.Zero(t, r.Pos(), "a failed read moved the cursor")
				require.True(t, fxErr != nil || allowed(err), "fxamacker accepts what LimbSlice rejects: %v", err)
				return
			}
			require.NoError(t, fxErr)
			require.Equal(t, want, got)
			require.Equal(t, rest, r.Remaining())
		})
	})
}

// TestSkipCountsAFeltAsALevel: a felt is an array, one nesting level to fxamacker, also when
// Skip passes it without decoding its heads. Here it is the 33rd level, under a map and a chain
// of tags, so fxamacker rejects the item.
func TestSkipCountsAFeltAsALevel(t *testing.T) {
	felt := []byte{
		0x84, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8,
		0x1b, 1, 2, 3, 4, 5, 6, 7, 8, 0x1b, 1, 2, 3, 4, 5, 6, 7, 8,
	}
	for tags := 28; tags <= 32; tags++ {
		item := append([]byte{0xa1, 0x61, 'a'}, bytes.Repeat([]byte{0xc1}, tags)...)
		item = append(item, felt...)
		_, fxErr := fxSkip(item)
		r := fast.NewReader(wrapped(item))
		err := r.Skip()
		require.Equalf(t, fxErr == nil, err == nil, "%d tags: fx=%v fast=%v", tags, fxErr, err)
	}
}
