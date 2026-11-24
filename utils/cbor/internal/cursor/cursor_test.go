package cursor_test

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

// What the readers refuse is in refusals_test.go.

func TestPeek(t *testing.T) {
	data := []byte{0x18, 0x2a}
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
		c := cursor.NewCursor([]byte{0xc2, 0x41, 0x01})

		tag, err := c.Tag()
		require.NoError(t, err)
		require.Equal(t, uint64(2), tag)

		content, err := c.Bytes()
		require.NoError(t, err)
		require.Equal(t, []byte{0x01}, content)
		require.Zero(t, c.Len())
	})

	t.Run("reads a tag number of several bytes", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0xda, 0x00, 0x01, 0x00, 0x00})

		tag, err := c.Tag()
		require.NoError(t, err)
		require.Equal(t, uint64(65536), tag)
		require.Zero(t, c.Len())
	})
}

func TestArrayLenAndMapLen(t *testing.T) {
	t.Run("array returns the element count", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x82, 0x01, 0x02})

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
		c := cursor.NewCursor([]byte{0xa2, 0x01, 0x02, 0x03, 0x04})

		n, err := c.MapLen()
		require.NoError(t, err)
		require.Equal(t, 2, n)
		require.Equal(t, 1, c.Pos())
	})

	t.Run("a count of several bytes larger than the input is refused", func(t *testing.T) {
		c := cursor.NewCursor([]byte{0x9a, 0xff, 0xff, 0xff, 0xff, 0x01})

		_, err := c.ArrayLen()
		require.ErrorIs(t, err, cursor.ErrTruncated)
		require.Zero(t, c.Pos())
	})
}

func openArray(c *cursor.Cursor) error {
	_, err := c.ArrayLen()
	return err
}

func openMap(c *cursor.Cursor) error {
	_, err := c.MapLen()
	return err
}

// openUntilRefused opens one level at a time over data, using each of openers in turn, until one is
// refused, and returns how many were opened. The refusal must be ErrDepth and consume nothing.
func openUntilRefused(t *testing.T, data []byte, openers ...func(*cursor.Cursor) error) int {
	t.Helper()
	c := cursor.NewCursor(data)
	for level := range 200 {
		pos := c.Pos()
		err := openers[level%len(openers)](&c)
		if err == nil {
			continue
		}
		require.ErrorIs(t, err, cursor.ErrDepth)
		require.Equal(t, pos, c.Pos())
		return level
	}
	require.FailNow(t, "nesting was never refused")
	return 0
}

// nested is depth arrays, each holding the next, around leaf.
func nested(depth int, leaf ...byte) []byte {
	return append(bytes.Repeat([]byte{0x81}, depth), leaf...)
}

// openable returns how many arrays c can open one inside the next, which is the nesting limit less
// the levels c has open. It works on a copy.
func openable(c cursor.Cursor) int {
	n := 0
	for openArray(&c) == nil {
		n++
	}
	return n
}

// nestingLimit returns how many levels a fresh Cursor can open.
func nestingLimit(t *testing.T) int {
	t.Helper()
	limit := openUntilRefused(t, nested(200, 0x01), openArray)
	require.Positive(t, limit)
	return limit
}

// openLevels opens n arrays on c.
func openLevels(t *testing.T, c *cursor.Cursor, n int) {
	t.Helper()
	for range n {
		require.NoError(t, openArray(c))
	}
}

func TestNestingLimit(t *testing.T) {
	arrays := nested(200, 0x01)
	maps := append(bytes.Repeat([]byte{0xa1}, 200), 0x01, 0x02) // each the key of the next
	mixed := append(bytes.Repeat([]byte{0x81, 0xa1}, 100), 0x01, 0x02)

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
		for _, head := range []byte{0xf6, 0xf7} {
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
