package cursor_test

import (
	"slices"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

func TestSkip(t *testing.T) {
	t.Run("consumes exactly the next item", func(t *testing.T) {
		for name, item := range map[string][]byte{
			"an integer":      {0x19, 0x01, 0x00},
			"a string":        {0x63, 'a', 'b', 'c'},
			"a byte string":   {0x42, 0x01, 0x02},
			"a simple value":  {0xf5},
			"a float":         {0xfb, 0x3f, 0xf1, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a},
			"an array":        {0x83, 0x01, 0x82, 0x02, 0x03, 0x82, 0x04, 0x05},
			"a map":           {0xa2, 0x61, 'a', 0x01, 0x61, 'b', 0x82, 0x02, 0x03},
			"an empty array":  {0x80},
			"a tag 4 or more": {0xc4, 0x82, 0x21, 0x19, 0x6a, 0xb3},
			"a felt":          {0x84, 0x01, 0x02, 0x03, 0x04},
		} {
			t.Run(name, func(t *testing.T) {
				c := cursor.NewCursor(slices.Concat(item, []byte{0xf6}))

				require.NoError(t, c.Skip())
				require.Equal(t, []byte{0xf6}, c.Remaining())
			})
		}
	})

	t.Run("does not check UTF-8", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x62, 0xff, 0xfe})

		require.NoError(t, c.Skip())
		require.Zero(t, c.Len())
	})

	t.Run("a refused item consumes nothing", func(t *testing.T) {
		tests := []struct {
			name string
			data []byte
			err  error
		}{
			{"an indefinite-length array", []byte{0x9f, 0x01, 0xff}, cursor.ErrUnsupported},
			{"an indefinite-length byte string", []byte{0x5f, 0x41, 0x01, 0xff}, cursor.ErrUnsupported},
			{"an indefinite-length map", []byte{0xbf, 0x01, 0x02, 0xff}, cursor.ErrUnsupported},
			{"tag 0", []byte{0xc0, 0x61, 'x'}, cursor.ErrUnsupported},
			{"tag 1", []byte{0xc1, 0x01}, cursor.ErrUnsupported},
			{"tag 2", []byte{0xc2, 0x41, 0x01}, cursor.ErrUnsupported},
			{"tag 3", []byte{0xc3, 0x41, 0x01}, cursor.ErrUnsupported},
			{"a map cut short", []byte{0xa2, 0x01, 0x02, 0x03}, cursor.ErrTruncated},
			{"a string longer than the input", []byte{0x63, 'a', 'b'}, cursor.ErrTruncated},
			{"an element cut short", []byte{0x82, 0x01, 0x19, 0x01}, cursor.ErrTruncated},
			{"a head that is not well formed", []byte{0x1c}, cursor.ErrSyntax},
			{"a simple value that should be short", []byte{0xf8, 0x10}, cursor.ErrSyntax},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				c := cursor.NewCursor(tt.data)

				require.ErrorIs(t, c.Skip(), tt.err)
				require.Zero(t, c.Pos())
			})
		}
	})

	t.Run("an item inside another is refused with the whole item", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x82, 0x01, 0x9f, 0xff})

		require.ErrorIs(t, c.Skip(), cursor.ErrUnsupported)
		require.Zero(t, c.Pos())
	})

	t.Run("nesting past the limit is refused and consumes nothing", func(t *testing.T) {
		limit := nestingLimit(t)

		c := cursor.NewCursor(nested(limit, 0x01))
		require.NoError(t, c.Skip())
		require.Zero(t, c.Len())

		c = cursor.NewCursor(nested(limit+1, 0x01))
		require.ErrorIs(t, c.Skip(), cursor.ErrDepth)
		require.Zero(t, c.Pos())
	})

	t.Run("the levels already open count toward the limit", func(t *testing.T) {
		limit := nestingLimit(t)

		c := cursor.NewCursor(nested(limit, 0x01)) // one level, then limit-1 more
		openLevels(t, &c, 1)
		require.NoError(t, c.Skip())
		require.Zero(t, c.Len())

		c = cursor.NewCursor(nested(limit+1, 0x01)) // one level, then limit more
		openLevels(t, &c, 1)
		pos := c.Pos()
		require.ErrorIs(t, c.Skip(), cursor.ErrDepth)
		require.Equal(t, pos, c.Pos())
	})
}
