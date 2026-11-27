package cursor_test

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor/internal/cursor"
	"github.com/stretchr/testify/require"
)

// Helpers for the nesting limit, shared by the tests of every reader that opens a level. They find
// the limit by opening levels until one is refused, so no test depends on its value.

// deeperThanLimit is more levels than the nesting limit allows. Inputs built to reach the limit are
// this deep, and openUntilRefused stops after this many levels.
const deeperThanLimit = 200

// nested is depth arrays of one element, each holding the next, around leaf.
func nested(depth int, leaf ...byte) []byte {
	return append(bytes.Repeat([]byte{0x81}, depth), leaf...)
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
	for level := range deeperThanLimit {
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

// nestingLimit returns how many levels a fresh Cursor can open.
func nestingLimit(t *testing.T) int {
	t.Helper()
	limit := openUntilRefused(t, nested(deeperThanLimit, 0x01), openArray)
	require.Positive(t, limit)
	return limit
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

// openLevels opens n arrays on c.
func openLevels(t *testing.T, c *cursor.Cursor, n int) {
	t.Helper()
	for range n {
		require.NoError(t, openArray(c))
	}
}
