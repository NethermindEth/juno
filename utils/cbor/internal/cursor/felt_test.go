package cursor_test

import (
	"slices"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

// A felt is an array of four unsigned integers, the words of its Montgomery form, each written as
// short as its value allows. Juno writes every word on eight bytes.
func TestLimbs(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want [cursor.Limbs]uint64
	}{
		{"one byte words", []byte{0x84, 0x01, 0x02, 0x03, 0x04}, [4]uint64{1, 2, 3, 4}},
		{
			"words of every width",
			[]byte{
				0x84, 0x17, 0x18, 0x18, 0x19, 0x01, 0x00, 0x1a, 0x00, 0x01, 0x00, 0x00,
			},
			[4]uint64{23, 24, 256, 65536},
		},
		{
			"eight byte words",
			[]byte{
				0x84,
				0x1b, 0, 0, 0, 0, 0, 0, 0, 1,
				0x1b, 0, 0, 0, 0, 0, 0, 1, 0,
				0x1b, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
				0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
			[4]uint64{1, 256, 0x0102030405060708, 0xffffffffffffffff},
		},
		{"a shorter array leaves the rest zero", []byte{0x82, 0x05, 0x06}, [4]uint64{5, 6, 0, 0}},
		{"a longer array is cut off", []byte{0x85, 0x01, 0x02, 0x03, 0x04, 0x05}, [4]uint64{1, 2, 3, 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cursor.NewCursor(slices.Concat(tt.data, []byte{0xf6})) // null, to see where the felt ends
			var got [cursor.Limbs]uint64
			require.NoError(t, c.Limbs(&got))
			require.Equal(t, tt.want, got)
			require.Equal(t, []byte{0xf6}, c.Remaining())
		})
	}
}

func TestLimbsRejects(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		err  error
	}{
		{"not an array", []byte{0x01}, cursor.ErrType},
		{"a negative word", []byte{0x84, 0x01, 0x20, 0x03, 0x04}, cursor.ErrType},
		{"a word of another type", []byte{0x84, 0x01, 0x61, 'a', 0x03, 0x04}, cursor.ErrType},
		{"a tag around a word", []byte{0x84, 0x01, 0xc4, 0x02, 0x03, 0x04}, cursor.ErrUnsupported},
		{"too few words", []byte{0x84, 0x01, 0x02}, cursor.ErrTruncated},
		{"an indefinite-length array", []byte{0x9f, 0x01, 0x02, 0x03, 0x04, 0xff}, cursor.ErrUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cursor.NewCursor(tt.data)
			got := [cursor.Limbs]uint64{9, 9, 9, 9}
			require.ErrorIs(t, c.Limbs(&got), tt.err)
			require.Equal(t, [cursor.Limbs]uint64{9, 9, 9, 9}, got, "dst is written only on success")
			require.Zero(t, c.Pos(), "nothing is consumed on error")
		})
	}
}

func TestLimbSlice(t *testing.T) {
	type felt [cursor.Limbs]uint64
	felts, err := func() ([]felt, error) {
		c := cursor.NewCursor([]byte{0x82, 0x84, 0x01, 0x02, 0x03, 0x04, 0x84, 0x05, 0x06, 0x07, 0x08})
		return cursor.LimbSlice[felt](&c)
	}()
	require.NoError(t, err)
	require.Equal(t, []felt{{1, 2, 3, 4}, {5, 6, 7, 8}}, felts)

	c := cursor.NewCursor([]byte{0xf6})
	felts, err = cursor.LimbSlice[felt](&c)
	require.NoError(t, err)
	require.Nil(t, felts, "null is a nil slice")

	c = cursor.NewCursor([]byte{0x80})
	felts, err = cursor.LimbSlice[felt](&c)
	require.NoError(t, err)
	require.NotNil(t, felts)
	require.Empty(t, felts)

	// A count the input cannot hold is refused before anything is allocated.
	c = cursor.NewCursor([]byte{0x9a, 0xff, 0xff, 0xff, 0xff, 0x84, 0x01, 0x02, 0x03, 0x04})
	_, err = cursor.LimbSlice[felt](&c)
	require.ErrorIs(t, err, cursor.ErrUnsupported)
	require.Zero(t, c.Pos())
}
