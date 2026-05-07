package pruner

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOldestRetainedBlock(t *testing.T) {
	t.Run("empty database returns ErrKeyNotFound", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)

		_, err := OldestRetainedBlock(database)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("returns lowest block number with commitments", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)

		for i := uint64(5); i <= 7; i++ {
			testutils.StoreBlock(t, database, i)
		}

		num, err := OldestRetainedBlock(database)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), num)
	})
}

func TestDeleteTransactionHashReverseLookups(t *testing.T) {
	database := testutils.NewPebbleTestDB(t)

	blocks := make([]*testutils.StoredBlock, 3)
	for i := range uint64(3) {
		blocks[i] = testutils.StoreBlock(t, database, i)
	}

	testutils.WithBatch(t, database, func(batch db.Batch) error {
		return deleteTransactionHashReverseLookups(database, batch, 1)
	})

	// Block 1's tx hash reverse lookups (bucket 9) and L1 handler msg hash
	// (bucket 24) should be gone.
	for _, txHash := range blocks[1].TxHashes {
		_, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(
			database,
			(*felt.TransactionHash)(txHash),
		)
		assert.Error(t, err)
	}
	for _, msgHash := range blocks[1].L1HandlerMsgHashes {
		_, err := core.GetL1HandlerTxnHashByMsgHash(database, msgHash)
		assert.Error(t, err)
	}

	// Header hash → number lookup is *not* this function's responsibility,
	// so it must still be present for block 1.
	assert.True(t, testutils.BlockHeaderHashExists(database, blocks[1].Header.Hash))

	// Neighbouring blocks must be untouched.
	for _, b := range []*testutils.StoredBlock{blocks[0], blocks[2]} {
		for _, txHash := range b.TxHashes {
			_, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(
				database,
				(*felt.TransactionHash)(txHash),
			)
			assert.NoError(t, err)
		}
		for _, msgHash := range b.L1HandlerMsgHashes {
			_, err := core.GetL1HandlerTxnHashByMsgHash(database, msgHash)
			assert.NoError(t, err)
		}
	}
}

func TestPruneAggregatedBloomFiltersUpto(t *testing.T) {
	n := core.NumBlocksPerFilter

	writeFilters := func(t *testing.T, database db.KeyValueWriter) {
		t.Helper()
		for i := range uint64(3) {
			filter := core.NewAggregatedFilter(i * n)
			require.NoError(t, core.WriteAggregatedBloomFilter(database, &filter))
		}
	}

	bloomFilterExists := func(database db.KeyValueReader, from, to uint64) bool {
		_, err := core.GetAggregatedBloomFilter(database, from, to)
		return err == nil
	}

	t.Run("deletes filters fully below the boundary", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		writeFilters(t, database)

		testutils.WithBatch(t, database, func(batch db.Batch) error {
			return pruneAggregatedBloomFiltersUpto(batch, 2*n)
		})

		assert.False(t, bloomFilterExists(database, 0, n-1))
		assert.False(t, bloomFilterExists(database, n, 2*n-1))
		assert.True(t, bloomFilterExists(database, 2*n, 3*n-1))
	})

	t.Run("partial range does not delete the containing filter", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		writeFilters(t, database)

		testutils.WithBatch(t, database, func(batch db.Batch) error {
			return pruneAggregatedBloomFiltersUpto(batch, n/2)
		})

		assert.True(t, bloomFilterExists(database, 0, n-1))
		assert.True(t, bloomFilterExists(database, n, 2*n-1))
		assert.True(t, bloomFilterExists(database, 2*n, 3*n-1))
	})

	t.Run("rangeEndExclusive at filter boundary keeps that filter", func(t *testing.T) {
		// rangeEndExclusive = n means filter [0, n-1] is fully below
		// (to = n-1 < n) and filter [n, 2n-1] starts at n, so it's kept.
		database := testutils.NewPebbleTestDB(t)
		writeFilters(t, database)

		testutils.WithBatch(t, database, func(batch db.Batch) error {
			return pruneAggregatedBloomFiltersUpto(batch, n)
		})

		assert.False(t, bloomFilterExists(database, 0, n-1))
		assert.True(t, bloomFilterExists(database, n, 2*n-1))
		assert.True(t, bloomFilterExists(database, 2*n, 3*n-1))
	})

	t.Run("no-op when range below one filter span", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)

		testutils.WithBatch(t, database, func(batch db.Batch) error {
			return pruneAggregatedBloomFiltersUpto(batch, 0)
		})
	})
}

// TestPruneUpto verifies the complete prune lifecycle with realistic block
// counts: what survives, what gets deleted, and the two intentional
// carve-outs (BlockHashLag headers and the single hash→number mapping at
// endExclusive-1).
//
// Setup: 30 blocks (0..29). BlockHashLag = 10. Call PruneUpto(20).
//
// Expected after the call:
//
//	Blocks [0, 10):  fully pruned (below the lag floor).
//	Blocks [10, 20): commitments / state updates / txs / tx-hash lookups /
//	                 state history pruned. Headers KEPT (lag carve-out).
//	                 Hash→number mapping KEPT only for block 19; pruned
//	                 for blocks 10..18.
//	Blocks [20, 30): untouched.
func TestPruneUpto(t *testing.T) {
	const totalBlocks uint64 = 30
	const endExclusive uint64 = 20
	lag := core.BlockHashLag
	require.GreaterOrEqual(t, endExclusive, lag)

	database := testutils.NewPebbleTestDB(t)
	blocks := make([]*testutils.StoredBlock, totalBlocks)
	for i := range totalBlocks {
		blocks[i] = testutils.StoreBlock(t, database, i)
	}
	pruned, oldestKept, err := PruneUpto(
		t.Context(),
		database,
		endExclusive,
		defaultTargetBatchByteSize,
	)
	require.NoError(t, err)
	assert.Equal(t, endExclusive, pruned)
	assert.Equal(t, endExclusive, oldestKept)

	testutils.AssertPostPruneState(t, database, blocks, endExclusive, lag)
}

func TestPruneUpto_NoOp(t *testing.T) {
	t.Run("empty database returns 0", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		pruned, oldestKept, err := PruneUpto(t.Context(), database, 20, defaultTargetBatchByteSize)
		require.NoError(t, err)
		assert.Zero(t, pruned)
		assert.Zero(t, oldestKept)
	})

	cases := []struct {
		name           string
		chainStart     uint64
		endExclusive   uint64
		wantOldestKept uint64
	}{
		{"endExclusive at oldestKept", 5, 5, 5},
		{"endExclusive below oldestKept", 10, 5, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is a no-op", func(t *testing.T) {
			database := testutils.NewPebbleTestDB(t)
			blocks := make([]*testutils.StoredBlock, 30)
			for i := tc.chainStart; i < 30; i++ {
				blocks[i] = testutils.StoreBlock(t, database, i)
			}
			pruned, oldestKept, err := PruneUpto(
				t.Context(),
				database,
				tc.endExclusive,
				defaultTargetBatchByteSize,
			)
			require.NoError(t, err)
			assert.Zero(t, pruned)
			assert.Equal(t, tc.wantOldestKept, oldestKept)
			for i := tc.chainStart; i < 30; i++ {
				testutils.AssertBlockExists(t, database, blocks[i])
			}
		})
	}
}

// cancelAfterGets wraps a KeyValueStore and cancels the supplied context
// after the n-th Get call. pruneHashKeyedUpto issues exactly one Get per
// loop iteration (core.GetStateUpdateByBlockNum), so this lets a test
// stop the sweep at a precise block boundary.
type cancelAfterGets struct {
	db.KeyValueStore
	cancel context.CancelFunc
	after  int
	count  int
}

func (c *cancelAfterGets) Get(key []byte, cb func([]byte) error) error {
	c.count++
	if c.count == c.after {
		c.cancel()
	}
	return c.KeyValueStore.Get(key, cb)
}

// TestPruneUpto_ResumeAfterMidwayCancel verifies that cancelling PruneUpto
// after some blocks have been processed and re-running picks up exactly
// where the first call stopped — no block is processed twice, none is
// skipped. Asserted by summing the per-call blocksPruned counts.
func TestPruneUpto_ResumeAfterMidwayCancel(t *testing.T) {
	const totalBlocks uint64 = 30
	const endExclusive uint64 = 25
	const cancelAfter = 10 // cancel after the 10th state-update Get

	database := testutils.NewPebbleTestDB(t)
	blocks := make([]*testutils.StoredBlock, totalBlocks)
	for i := range totalBlocks {
		blocks[i] = testutils.StoreBlock(t, database, i)
	}

	ctx, cancel := context.WithCancel(t.Context())
	wrapped := &cancelAfterGets{KeyValueStore: database, cancel: cancel, after: cancelAfter}

	pruned1, oldestKept1, err := PruneUpto(ctx, wrapped, endExclusive, defaultTargetBatchByteSize)
	require.NoError(t, err)
	require.Greater(t, pruned1, uint64(0), "first call should make progress before cancel")
	require.Less(t, pruned1, endExclusive, "first call must not finish the full window")
	assert.Equal(t, pruned1, oldestKept1, "oldestKept after partial prune == pruned (start was 0)")

	pruned2, oldestKept2, err := PruneUpto(
		t.Context(), database, endExclusive, defaultTargetBatchByteSize,
	)
	require.NoError(t, err)
	assert.Equal(t, endExclusive, oldestKept2)

	assert.Equal(t, endExclusive, pruned1+pruned2,
		"sum of blocks pruned across both calls must equal the full window — "+
			"any duplicate or skipped block would break this invariant")

	// End-state: blocks below the lag floor are gone, blocks ≥ endExclusive intact.
	for i := range endExclusive - core.BlockHashLag {
		testutils.AssertBlockPruned(t, database, blocks[i])
	}
	for i := endExclusive; i < totalBlocks; i++ {
		testutils.AssertBlockExists(t, database, blocks[i])
	}
}

// TestPruneUpto_RollingCarveOut verifies that across two consecutive
// PruneUpto calls, the hash→number carve-out rolls forward correctly:
// after each call exactly one extra mapping survives below oldestKept,
// and the previous extra is cleaned up by the next call's start-1 sweep.
func TestPruneUpto_RollingCarveOut(t *testing.T) {
	const totalBlocks uint64 = 40
	lag := core.BlockHashLag

	database := testutils.NewPebbleTestDB(t)
	blocks := make([]*testutils.StoredBlock, totalBlocks)
	for i := range totalBlocks {
		blocks[i] = testutils.StoreBlock(t, database, i)
	}

	// Call 1: prune up to 20.
	pruned, oldestKept, err := PruneUpto(t.Context(), database, 20, defaultTargetBatchByteSize)
	require.NoError(t, err)
	assert.Equal(t, uint64(20), pruned)
	assert.Equal(t, uint64(20), oldestKept)
	testutils.AssertPostPruneState(t, database, blocks, 20, lag)

	// Call 2: prune up to 30. OldestRetainedBlock returns 20 now, so we prune 10 more.
	pruned, oldestKept, err = PruneUpto(context.Background(), database, 30, defaultTargetBatchByteSize)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), pruned)
	assert.Equal(t, uint64(30), oldestKept)
	testutils.AssertPostPruneState(t, database, blocks, 30, lag)
}

// TestPruneBlockDataUpto exercises the range-tombstone half on its own,
// confirming the BlockHashLag carve-out for headers without the per-block
// hash-keyed work that PruneUpto wraps around it.
func TestPruneBlockDataUpto(t *testing.T) {
	const totalBlocks uint64 = 30
	const endExclusive uint64 = 20
	lag := core.BlockHashLag

	database := testutils.NewPebbleTestDB(t)
	for i := range totalBlocks {
		testutils.StoreBlock(t, database, i)
	}

	testutils.WithBatch(t, database, func(batch db.Batch) error {
		return PruneBlockDataUpto(batch, endExclusive)
	})

	// Headers below the lag floor: gone.
	for i := range endExclusive - lag {
		assert.False(t, testutils.BlockHeaderExists(database, i),
			"header at block %d should be deleted", i)
	}
	// Headers in the lag window: kept.
	for i := endExclusive - lag; i < endExclusive; i++ {
		assert.True(t, testutils.BlockHeaderExists(database, i),
			"header at block %d should be kept (lag carve-out)", i)
	}
	// Headers beyond endExclusive: untouched.
	for i := endExclusive; i < totalBlocks; i++ {
		assert.True(t, testutils.BlockHeaderExists(database, i),
			"header at block %d should be untouched", i)
	}

	// Number-keyed buckets without a lag: deleted across the full window.
	for i := range endExclusive {
		assert.False(t, testutils.BlockCommitmentsExist(database, i),
			"commitments at block %d should be deleted", i)
		assert.False(t, testutils.StateUpdateExists(database, i),
			"state update at block %d should be deleted", i)
		assert.False(t, testutils.TransactionsExist(database, i),
			"transactions at block %d should be deleted", i)
	}
	for i := endExclusive; i < totalBlocks; i++ {
		assert.True(t, testutils.BlockCommitmentsExist(database, i),
			"commitments at block %d should be untouched", i)
		assert.True(t, testutils.StateUpdateExists(database, i),
			"state update at block %d should be untouched", i)
		assert.True(t, testutils.TransactionsExist(database, i),
			"transactions at block %d should be untouched", i)
	}
}
