package cursor_test

import (
	"slices"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

func TestSkip(t *testing.T) {
	// A felt as Juno writes it, which Skip passes without reading its heads.
	wideFelt := []byte{
		0x84,                         // array of 4
		0x1b, 0, 0, 0, 0, 0, 0, 0, 1, // 1, in a head and eight bytes
		0x1b, 0, 0, 0, 0, 0, 0, 0, 2, // 2
		0x1b, 0, 0, 0, 0, 0, 0, 0, 3, // 3
		0x1b, 0, 0, 0, 0, 0, 0, 0, 4, // 4
	}

	t.Run("consumes exactly the next item", func(t *testing.T) {
		for name, item := range map[string][]byte{
			"an integer":     {0x19, 0x01, 0x00},    // 256, in a head and two bytes
			"a string":       {0x63, 'a', 'b', 'c'}, // the text "abc"
			"a byte string":  {0x42, 0x01, 0x02},    // a byte string of two bytes
			"a simple value": {0xf5},                // true
			// 1.1, in a head and eight bytes
			"a float": {0xfb, 0x3f, 0xf1, 0x99, 0x99, 0x99, 0x99, 0x99, 0x9a},
			"an array": {
				0x83,             // array of 3
				0x01,             // 1
				0x82, 0x02, 0x03, // [2, 3]
				0x82, 0x04, 0x05, // [4, 5]
			},
			"a map": {
				0xa2,            // map of 2 pairs
				0x61, 'a', 0x01, // "a": 1
				0x61, 'b', 0x82, 0x02, 0x03, // "b": [2, 3]
			},
			"an empty array":             {0x80},
			"a tag 4 or more":            {0xc4, 0x82, 0x21, 0x19, 0x6a, 0xb3}, // tag 4, [-2, 27315]
			"a felt of one-byte limbs":   {0x84, 0x01, 0x02, 0x03, 0x04},       // [1, 2, 3, 4]
			"a felt of eight-byte limbs": wideFelt,
			// An array of 2, both felts of eight-byte limbs.
			"an array of felts of eight-byte limbs": slices.Concat([]byte{0x82}, wideFelt, wideFelt),
		} {
			t.Run(name, func(t *testing.T) {
				c := cursor.NewCursor(slices.Concat(item, []byte{0xf6})) // null, to see where the item ends

				require.NoError(t, c.Skip())
				require.Equal(t, []byte{0xf6}, c.Remaining())
			})
		}
	})

	t.Run("does not check UTF-8", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x62, 0xff, 0xfe}) // a text string of two bytes that are not UTF-8

		require.NoError(t, c.Skip())
		require.Zero(t, c.Len())
	})

	t.Run("a refused item consumes nothing", func(t *testing.T) {
		tests := []struct {
			name string
			data []byte
			err  error
		}{
			// [_ 1]
			{"an indefinite-length array", []byte{0x9f, 0x01, 0xff}, cursor.ErrUnsupported},
			// (_ h'01')
			{"an indefinite-length byte string", []byte{0x5f, 0x41, 0x01, 0xff}, cursor.ErrUnsupported},
			// {_ 1: 2}
			{"an indefinite-length map", []byte{0xbf, 0x01, 0x02, 0xff}, cursor.ErrUnsupported},
			{"tag 0", []byte{0xc0, 0x61, 'x'}, cursor.ErrUnsupported},  // tag 0, "x"
			{"tag 1", []byte{0xc1, 0x01}, cursor.ErrUnsupported},       // tag 1, 1
			{"tag 2", []byte{0xc2, 0x41, 0x01}, cursor.ErrUnsupported}, // tag 2, h'01'
			{"tag 3", []byte{0xc3, 0x41, 0x01}, cursor.ErrUnsupported}, // tag 3, h'01'
			// A map of 2 pairs with three items.
			{"a map cut short", []byte{0xa2, 0x01, 0x02, 0x03}, cursor.ErrTruncated},
			// A text string of three bytes with two.
			{"a string longer than the input", []byte{0x63, 'a', 'b'}, cursor.ErrTruncated},
			// [1, an integer of two bytes with one]
			{"an element cut short", []byte{0x82, 0x01, 0x19, 0x01}, cursor.ErrTruncated},
			// An integer head with the reserved additional information 28.
			{"a head that is not well formed", []byte{0x1c}, cursor.ErrSyntax},
			// The simple value 16, in two bytes.
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
		c := cursor.NewCursor([]byte{0x82, 0x01, 0x9f, 0xff}) // [1, an indefinite-length array]

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

	// A felt is an array, so it takes a level whatever its form.
	t.Run("a felt of eight-byte limbs takes a level", func(t *testing.T) {
		limit := nestingLimit(t)

		t.Run("inside the item skipped", func(t *testing.T) {
			c := cursor.NewCursor(nested(limit-1, wideFelt...))
			require.NoError(t, c.Skip())
			require.Zero(t, c.Len())

			c = cursor.NewCursor(nested(limit, wideFelt...))
			require.ErrorIs(t, c.Skip(), cursor.ErrDepth)
			require.Zero(t, c.Pos())
		})

		t.Run("inside the levels already open", func(t *testing.T) {
			c := cursor.NewCursor(nested(limit-1, wideFelt...))
			openLevels(t, &c, limit-1)
			require.NoError(t, c.Skip())
			require.Zero(t, c.Len())

			c = cursor.NewCursor(nested(limit, wideFelt...))
			openLevels(t, &c, limit)
			pos := c.Pos()
			require.ErrorIs(t, c.Skip(), cursor.ErrDepth)
			require.Equal(t, pos, c.Pos())
		})
	})
}
