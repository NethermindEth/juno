package cursor_test

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

// What the readers refuse is in refusals_test.go, and the nesting helpers in helpers_test.go.

func TestPeekDoesNotConsumeCursor(t *testing.T) {
	data := []byte{0x18, 0x2a} // the integer 42, in a head and one byte
	c := cursor.NewCursor(data)

	for range 2 {
		head, err := c.Peek()
		require.NoError(t, err)
		require.Equal(t, byte(0x18), head)
		require.Zero(t, c.Pos())
		require.Equal(t, len(data), c.Len())
	}
}

func TestTag(t *testing.T) {
	t.Run("reads the tag number and leaves the content", func(t *testing.T) {
		c := cursor.NewCursor([]byte{
			0xc2,       // tag 2
			0x41, 0x01, // a byte string of one byte
		})

		tag, err := c.Tag()
		require.NoError(t, err)
		require.Equal(t, uint64(2), tag)

		content, err := c.Bytes()
		require.NoError(t, err)
		require.Equal(t, []byte{0x01}, content)
		require.Zero(t, c.Len())
	})

	t.Run("reads a tag number of several bytes", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0xda, 0x00, 0x01, 0x00, 0x00}) // tag 65536, in a head and four bytes

		tag, err := c.Tag()
		require.NoError(t, err)
		require.Equal(t, uint64(65536), tag)
		require.Zero(t, c.Len())
	})
}

func TestArrayLenAndMapLen(t *testing.T) {
	t.Run("array returns the element count", func(t *testing.T) {
		c := cursor.NewCursor([]byte{
			0x82, // array of 2
			0x01, // 1
			0x02, // 2
		})

		n, err := c.ArrayLen()
		require.NoError(t, err)
		require.Equal(t, 2, n)
		require.Equal(t, 1, c.Pos())

		for range n {
			_, err := c.Uint64()
			require.NoError(t, err)
		}
		c.Leave()
		require.Zero(t, c.Len())
	})

	t.Run("map returns the pair count", func(t *testing.T) {
		c := cursor.NewCursor([]byte{
			0xa2, // map of 2 pairs
			0x01, // key 1
			0x02, // value 2
			0x03, // key 3
			0x04, // value 4
		})

		n, err := c.MapLen()
		require.NoError(t, err)
		require.Equal(t, 2, n)
		require.Equal(t, 1, c.Pos())
	})

	t.Run("a count of several bytes larger than the input is refused", func(t *testing.T) {
		c := cursor.NewCursor([]byte{
			0x9a, 0xff, 0xff, 0xff, 0xff, // array of 2^32-1, the count in four bytes
			0x01, // a single element
		})

		_, err := c.ArrayLen()
		require.ErrorIs(t, err, cursor.ErrTruncated)
		require.Zero(t, c.Pos())
	})
}

func TestNestingLimit(t *testing.T) {
	arrays := nested(deeperThanLimit, 0x01)
	// Maps of one pair, each the key of the next, around the pair 1: 2.
	maps := append(bytes.Repeat([]byte{0xa1}, deeperThanLimit), 0x01, 0x02)
	// An array of one element and a map of one pair in turn, around the pair 1: 2.
	mixed := append(bytes.Repeat([]byte{0x81, 0xa1}, deeperThanLimit/2), 0x01, 0x02)

	t.Run("arrays and maps are refused past a limit and consume nothing", func(t *testing.T) {
		require.Positive(t, openUntilRefused(t, arrays, openArray))
		require.Positive(t, openUntilRefused(t, maps, openMap))
	})

	t.Run("arrays and maps count toward the same limit", func(t *testing.T) {
		require.Equal(t,
			openUntilRefused(t, arrays, openArray),
			openUntilRefused(t, mixed, openArray, openMap),
		)
	})

	t.Run("Leave makes room to open again", func(t *testing.T) {
		c := cursor.NewCursor(arrays)
		openLevels(t, &c, nestingLimit(t))
		require.ErrorIs(t, openArray(&c), cursor.ErrDepth)

		c.Leave()
		require.NoError(t, openArray(&c))
		require.ErrorIs(t, openArray(&c), cursor.ErrDepth)
	})
}

func TestNull(t *testing.T) {
	t.Run("consumes null and undefined", func(t *testing.T) {
		for _, head := range []byte{0xf6, 0xf7} { // null, undefined
			c := cursor.NewCursor([]byte{head, 0x01})

			require.True(t, c.Null())
			require.Equal(t, 1, c.Pos())
		}
	})

	t.Run("leaves any other item, and the end of the input", func(t *testing.T) {
		for _, data := range [][]byte{
			nil,
			{0x00},       // an integer
			{0xf4},       // false
			{0x80},       // an empty array
			{0xf9, 0x00}, // a float
		} {
			c := cursor.NewCursor(data)

			require.False(t, c.Null())
			require.Zero(t, c.Pos())
		}
	})
}

func TestScratch(t *testing.T) {
	c := cursor.NewCursor(nil)

	*(*[4]uint64)(c.Scratch()) = [4]uint64{1, 2, 3, 4}

	require.Equal(t, [4]uint64{1, 2, 3, 4}, *(*[4]uint64)(c.Scratch()))
}
