package rpccore_test

import (
	"testing"

	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/require"
)

func TestPreConfirmedCursor(t *testing.T) {
	const id1, id2 = "round-1", "round-2"

	t.Run("starts from the first transaction", func(t *testing.T) {
		cursor := rpccore.NewPreConfirmedCursor()
		require.Equal(t, 0, cursor.Advance(100, id1, 3))
	})

	t.Run("a grown tip starts after the transactions already processed", func(t *testing.T) {
		cursor := rpccore.NewPreConfirmedCursor()
		require.Equal(t, 0, cursor.Advance(100, id1, 3))
		require.Equal(t, 3, cursor.Advance(100, id1, 5))
		require.Equal(t, 5, cursor.Advance(100, id1, 9))
	})

	t.Run("an unchanged tip processes nothing", func(t *testing.T) {
		cursor := rpccore.NewPreConfirmedCursor()
		require.Equal(t, 0, cursor.Advance(100, id1, 3))
		require.Equal(t, 3, cursor.Advance(100, id1, 3))
	})

	t.Run("a stale shorter tip processes nothing and keeps the highest count", func(t *testing.T) {
		cursor := rpccore.NewPreConfirmedCursor()
		require.Equal(t, 0, cursor.Advance(100, id1, 5))
		require.Equal(t, 2, cursor.Advance(100, id1, 2)) // nothing to process: from == txCount
		require.Equal(t, 5, cursor.Advance(100, id1, 7)) // the shorter tip did not lower the count
	})

	t.Run("an empty tip processes nothing", func(t *testing.T) {
		cursor := rpccore.NewPreConfirmedCursor()
		require.Equal(t, 0, cursor.Advance(100, id1, 0))
		require.Equal(t, 0, cursor.Advance(100, id1, 2))
	})

	t.Run("advancing the block starts again", func(t *testing.T) {
		cursor := rpccore.NewPreConfirmedCursor()
		require.Equal(t, 0, cursor.Advance(100, id1, 3))
		require.Equal(t, 0, cursor.Advance(101, id1, 2))
		// block 100 is no longer tracked, so it is processed again
		require.Equal(t, 0, cursor.Advance(100, id1, 3))
	})

	t.Run("a new round identifier at the same height starts again", func(t *testing.T) {
		cursor := rpccore.NewPreConfirmedCursor()
		require.Equal(t, 0, cursor.Advance(100, id1, 3))
		require.Equal(t, 0, cursor.Advance(100, id2, 2))
		require.Equal(t, 2, cursor.Advance(100, id2, 4))
	})

	t.Run("Clear forgets the current round", func(t *testing.T) {
		cursor := rpccore.NewPreConfirmedCursor()
		require.Equal(t, 0, cursor.Advance(100, id1, 3))
		cursor.Clear()
		require.Equal(t, 0, cursor.Advance(100, id1, 3))
	})

	t.Run("block zero with an empty identifier behaves like any other round", func(t *testing.T) {
		cursor := rpccore.NewPreConfirmedCursor()
		require.Equal(t, 0, cursor.Advance(0, "", 2))
		require.Equal(t, 2, cursor.Advance(0, "", 2))
		require.Equal(t, 0, cursor.Advance(1, "", 1))
		require.Equal(t, 0, cursor.Advance(0, "", 2)) // block 0 forgotten
	})
}
