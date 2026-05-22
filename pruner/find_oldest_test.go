package pruner_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindOldestBlockAtOrAfter exercises the binary-search primitive that
// powers the min-age floor's seed and per-tick refresh.
func TestFindOldestBlockAtOrAfter(t *testing.T) {
	// storeRange writes blocks [from, to) each with the given timestamp.
	storeRange := func(t *testing.T, database db.KeyValueStore, from, to, timestamp uint64) {
		t.Helper()
		for i := from; i < to; i++ {
			testutils.StoreBlockWithTimestamp(t, database, i, timestamp)
		}
	}

	t.Run("lower > upper returns ErrNoBlockInWindow", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		_, err := pruner.FindOldestBlockAtOrAfter(database, 10, 5, time.Unix(100, 0))
		require.ErrorIs(t, err, pruner.ErrNoBlockInWindow)
	})

	t.Run("all blocks ancient → returns ErrNoBlockInWindow", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		storeRange(t, database, 0, 20, 1) // all Timestamp=1
		_, err := pruner.FindOldestBlockAtOrAfter(database, 0, 19, time.Unix(1000, 0))
		require.ErrorIs(t, err, pruner.ErrNoBlockInWindow,
			"no block satisfies cutoff → explicit not-found error")
	})

	t.Run("all blocks fresh → returns lower", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		storeRange(t, database, 0, 20, 5000) // all Timestamp=5000
		got, err := pruner.FindOldestBlockAtOrAfter(database, 0, 19, time.Unix(1000, 0))
		require.NoError(t, err)
		assert.Equal(t, uint64(0), got, "lowest qualifying block is the lower bound")
	})

	t.Run("boundary in the middle", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		// Blocks 0..9 ancient (Timestamp=1), 10..19 fresh (Timestamp=5000).
		storeRange(t, database, 0, 10, 1)
		storeRange(t, database, 10, 20, 5000)
		got, err := pruner.FindOldestBlockAtOrAfter(database, 0, 19, time.Unix(1000, 0))
		require.NoError(t, err)
		assert.Equal(t, uint64(10), got, "first fresh block is the boundary")
	})

	t.Run("Timestamp equal to cutoff qualifies (>= not >)", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		storeRange(t, database, 0, 5, 1)
		testutils.StoreBlockWithTimestamp(t, database, 5, 1000) // exactly at cutoff
		storeRange(t, database, 6, 10, 5000)
		got, err := pruner.FindOldestBlockAtOrAfter(database, 0, 9, time.Unix(1000, 0))
		require.NoError(t, err)
		assert.Equal(t, uint64(5), got, "ts == cutoff must qualify (>=)")
	})

	t.Run("missing block in range propagates ErrKeyNotFound", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		// Block 5 NOT stored — caller passed a range that crosses a
		// pruned height; the search hits ErrKeyNotFound and surfaces it.
		storeRange(t, database, 0, 5, 5000)
		storeRange(t, database, 6, 10, 5000)
		_, err := pruner.FindOldestBlockAtOrAfter(database, 5, 9, time.Unix(1000, 0))
		require.ErrorIs(t, err, db.ErrKeyNotFound,
			"caller must pass a range of retained blocks; gaps surface as ErrKeyNotFound")
	})

	t.Run("single-element range [n, n]: block qualifies", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		testutils.StoreBlockWithTimestamp(t, database, 7, 5000)
		got, err := pruner.FindOldestBlockAtOrAfter(database, 7, 7, time.Unix(1000, 0))
		require.NoError(t, err)
		assert.Equal(t, uint64(7), got)
	})

	t.Run("single-element range [n, n]: block doesn't qualify", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		testutils.StoreBlockWithTimestamp(t, database, 7, 1)
		_, err := pruner.FindOldestBlockAtOrAfter(database, 7, 7, time.Unix(1000, 0))
		require.ErrorIs(t, err, pruner.ErrNoBlockInWindow)
	})

	t.Run("propagates non-ErrKeyNotFound DB errors", func(t *testing.T) {
		// Use a closed pebble DB to force a real error from GetBlockHeaderByNumber.
		database := testutils.NewPebbleTestDB(t)
		storeRange(t, database, 0, 10, 5000)
		require.NoError(t, database.Close())
		_, err := pruner.FindOldestBlockAtOrAfter(database, 0, 9, time.Unix(1000, 0))
		require.Error(t, err, "closed DB must surface as an error")
		assert.NotErrorIs(t, err, db.ErrKeyNotFound, "should be a real DB error, not ErrKeyNotFound")
		assert.NotErrorIs(t, err, pruner.ErrNoBlockInWindow, "should not be conflated with not-found")
	})
}
