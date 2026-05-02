package pruner

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOldestRetainedBlock(t *testing.T) {
	t.Run("empty database returns ErrKeyNotFound", func(t *testing.T) {
		database := newTestDB(t)

		_, err := OldestRetainedBlock(database)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("returns lowest block number with commitments", func(t *testing.T) {
		database := newTestDB(t)

		for i := uint64(5); i <= 7; i++ {
			storeBlock(t, database, i)
		}

		num, err := OldestRetainedBlock(database)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), num)
	})
}

func TestDeleteTransactionHashReverseLookups(t *testing.T) {
	database := newTestDB(t)

	blocks := make([]*storedBlock, 3)
	for i := range uint64(3) {
		blocks[i] = storeBlock(t, database, i)
	}

	withBatch(t, database, func(batch db.Batch) error {
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
	assert.True(t, blockHeaderHashExists(database, blocks[1].Header.Hash))

	// Neighbouring blocks must be untouched.
	for _, b := range []*storedBlock{blocks[0], blocks[2]} {
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
		database := newTestDB(t)
		writeFilters(t, database)

		withBatch(t, database, func(batch db.Batch) error {
			return pruneAggregatedBloomFiltersUpto(batch, 2*n)
		})

		assert.False(t, bloomFilterExists(database, 0, n-1))
		assert.False(t, bloomFilterExists(database, n, 2*n-1))
		assert.True(t, bloomFilterExists(database, 2*n, 3*n-1))
	})

	t.Run("partial range does not delete the containing filter", func(t *testing.T) {
		database := newTestDB(t)
		writeFilters(t, database)

		withBatch(t, database, func(batch db.Batch) error {
			return pruneAggregatedBloomFiltersUpto(batch, n/2)
		})

		assert.True(t, bloomFilterExists(database, 0, n-1))
		assert.True(t, bloomFilterExists(database, n, 2*n-1))
		assert.True(t, bloomFilterExists(database, 2*n, 3*n-1))
	})

	t.Run("rangeEndExclusive at filter boundary keeps that filter", func(t *testing.T) {
		// rangeEndExclusive = n means filter [0, n-1] is fully below
		// (to = n-1 < n) and filter [n, 2n-1] starts at n, so it's kept.
		database := newTestDB(t)
		writeFilters(t, database)

		withBatch(t, database, func(batch db.Batch) error {
			return pruneAggregatedBloomFiltersUpto(batch, n)
		})

		assert.False(t, bloomFilterExists(database, 0, n-1))
		assert.True(t, bloomFilterExists(database, n, 2*n-1))
		assert.True(t, bloomFilterExists(database, 2*n, 3*n-1))
	})

	t.Run("no-op when range below one filter span", func(t *testing.T) {
		database := newTestDB(t)

		withBatch(t, database, func(batch db.Batch) error {
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

	database := newTestDB(t)
	blocks := make([]*storedBlock, totalBlocks)
	for i := range totalBlocks {
		blocks[i] = storeBlock(t, database, i)
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

	// Subtests below are read-only assertions on the shared post-prune state.

	t.Run("blocks below the lag floor are completely gone", func(t *testing.T) {
		for i := range endExclusive - lag {
			assertBlockPruned(t, database, blocks[i])
		}
	})

	t.Run("blocks in the lag window keep their header", func(t *testing.T) {
		for i := endExclusive - lag; i < endExclusive; i++ { // 10..19
			assert.True(t, blockHeaderExists(database, i),
				"header at block %d must be kept (BlockHashLag carve-out)", i)
		}
	})

	t.Run("blocks in the lag window have all non-header data deleted", func(t *testing.T) {
		for i := endExclusive - lag; i < endExclusive; i++ { // 10..19
			assert.False(t, blockCommitmentsExist(database, i),
				"block %d commitments should be deleted", i)
			assert.False(t, stateUpdateExists(database, i),
				"block %d state update should be deleted", i)
			assert.False(t, transactionsExist(database, i),
				"block %d transactions should be deleted", i)
			for j, txHash := range blocks[i].TxHashes {
				_, err := core.TransactionBlockNumbersAndIndicesByHashBucket.Get(
					database,
					(*felt.TransactionHash)(txHash),
				)
				assert.Error(t, err, "block %d tx %d hash lookup should be deleted", i, j)
			}
			for _, msgHash := range blocks[i].L1HandlerMsgHashes {
				_, err := core.GetL1HandlerTxnHashByMsgHash(database, msgHash)
				assert.Error(t, err, "block %d L1 handler msg hash should be deleted", i)
			}
			assertStateHistoryPruned(t, database, i, blocks[i].StateUpdate)
		}
	})

	t.Run("hash→number carve-out preserved at endExclusive-1 only", func(t *testing.T) {
		for i := range endExclusive - 1 { // 0..18
			assert.False(t, blockHeaderHashExists(database, blocks[i].Header.Hash),
				"hash→number for block %d should be deleted", i)
		}
		assert.True(t, blockHeaderHashExists(database, blocks[endExclusive-1].Header.Hash))
	})

	t.Run("get_block_hash_syscall scenario", func(t *testing.T) {
		// After the prune, oldestKept = endExclusive (no commitments below it).
		// The syscall executes blocks in [endExclusive, endExclusive+lag) and
		// for each, reads the header at (current - BlockHashLag), which lands
		// somewhere in [endExclusive-lag, endExclusive). All those headers
		// must be readable.
		for i := endExclusive - lag; i < endExclusive; i++ {
			h, err := core.GetBlockHeaderByNumber(database, i)
			require.NoError(t, err, "syscall path: header at block %d must be readable", i)
			assert.Equal(t, i, h.Number)
		}
	})

	t.Run("StateAtBlockHash(oldestKept.parentHash) scenario", func(t *testing.T) {
		// oldestKept after prune is endExclusive=20. Its parent is block 19,
		// whose hash is blocks[19].Header.Hash. The chain must resolve:
		// hash → number (carve-out) → header (lag carve-out).
		parentHash := blocks[endExclusive-1].Header.Hash
		header, err := core.GetBlockHeaderByHash(database, parentHash)
		require.NoError(t, err)
		assert.Equal(t, endExclusive-1, header.Number)
	})

	t.Run("blocks beyond endExclusive untouched", func(t *testing.T) {
		for i := endExclusive; i < totalBlocks; i++ {
			assertBlockExists(t, database, blocks[i])
		}
	})
}

func TestPruneUpto_NoOp(t *testing.T) {
	t.Run("empty database returns 0", func(t *testing.T) {
		database := newTestDB(t)
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
			database := newTestDB(t)
			blocks := make([]*storedBlock, 30)
			for i := tc.chainStart; i < 30; i++ {
				blocks[i] = storeBlock(t, database, i)
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
				assertBlockExists(t, database, blocks[i])
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

	database := newTestDB(t)
	blocks := make([]*storedBlock, totalBlocks)
	for i := range totalBlocks {
		blocks[i] = storeBlock(t, database, i)
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
		assertBlockPruned(t, database, blocks[i])
	}
	for i := endExclusive; i < totalBlocks; i++ {
		assertBlockExists(t, database, blocks[i])
	}
}

// TestPruneUpto_RollingCarveOut verifies that across two consecutive
// PruneUpto calls, the hash→number carve-out rolls forward correctly:
// after each call exactly one extra mapping survives below oldestKept,
// and the previous extra is cleaned up by the next call's start-1 sweep.
func TestPruneUpto_RollingCarveOut(t *testing.T) {
	const totalBlocks uint64 = 40
	lag := core.BlockHashLag

	database := newTestDB(t)
	blocks := make([]*storedBlock, totalBlocks)
	for i := range totalBlocks {
		blocks[i] = storeBlock(t, database, i)
	}

	// Call 1: prune up to 20.
	pruned, oldestKept, err := PruneUpto(t.Context(), database, 20, defaultTargetBatchByteSize)
	require.NoError(t, err)
	assert.Equal(t, uint64(20), pruned)
	assert.Equal(t, uint64(20), oldestKept)

	// After call 1: only block 19 keeps its hash→number mapping.
	for i := range uint64(19) {
		assert.False(t, blockHeaderHashExists(database, blocks[i].Header.Hash),
			"after call 1: hash→number for block %d should be gone", i)
	}
	assert.True(t, blockHeaderHashExists(database, blocks[19].Header.Hash),
		"after call 1: block 19's hash→number is the carve-out")
	// And lag headers [10, 20) are kept.
	for i := uint64(20) - lag; i < 20; i++ {
		assert.True(t, blockHeaderExists(database, i),
			"after call 1: header at block %d should be in the lag window", i)
	}

	// Call 2: prune up to 30.
	pruned, oldestKept, err = PruneUpto(context.Background(), database, 30, defaultTargetBatchByteSize)
	require.NoError(t, err)
	// OldestRetainedBlock returns 20 now, so we prune 10 more.
	assert.Equal(t, uint64(10), pruned)
	assert.Equal(t, uint64(30), oldestKept)

	// After call 2: block 19's mapping has been swept by the start-1 cleanup.
	assert.False(t, blockHeaderHashExists(database, blocks[19].Header.Hash),
		"after call 2: block 19's mapping should have been cleaned up")
	for i := uint64(20); i < 29; i++ {
		assert.False(t, blockHeaderHashExists(database, blocks[i].Header.Hash),
			"after call 2: hash→number for block %d should be gone", i)
	}
	assert.True(t, blockHeaderHashExists(database, blocks[29].Header.Hash),
		"after call 2: block 29's hash→number is the new carve-out")

	// Header lag has rolled forward to [20, 30); the previous lag window
	// [10, 20) is now fully gone.
	for i := uint64(10); i < 20; i++ {
		assert.False(t, blockHeaderExists(database, i),
			"after call 2: header at block %d should be pruned (no longer in lag)", i)
	}
	for i := uint64(30) - lag; i < 30; i++ {
		assert.True(t, blockHeaderExists(database, i),
			"after call 2: header at block %d should be in the new lag window", i)
	}

	// Blocks beyond endExclusive still intact.
	for i := uint64(30); i < totalBlocks; i++ {
		assertBlockExists(t, database, blocks[i])
	}
}

// TestPruneBlockDataUpto exercises the range-tombstone half on its own,
// confirming the BlockHashLag carve-out for headers without the per-block
// hash-keyed work that PruneUpto wraps around it.
func TestPruneBlockDataUpto(t *testing.T) {
	const totalBlocks uint64 = 30
	const endExclusive uint64 = 20
	lag := core.BlockHashLag

	database := newTestDB(t)
	for i := range totalBlocks {
		storeBlock(t, database, i)
	}

	withBatch(t, database, func(batch db.Batch) error {
		return PruneBlockDataUpto(batch, endExclusive)
	})

	// Headers below the lag floor: gone.
	for i := range endExclusive - lag {
		assert.False(t, blockHeaderExists(database, i),
			"header at block %d should be deleted", i)
	}
	// Headers in the lag window: kept.
	for i := endExclusive - lag; i < endExclusive; i++ {
		assert.True(t, blockHeaderExists(database, i),
			"header at block %d should be kept (lag carve-out)", i)
	}
	// Headers beyond endExclusive: untouched.
	for i := endExclusive; i < totalBlocks; i++ {
		assert.True(t, blockHeaderExists(database, i),
			"header at block %d should be untouched", i)
	}

	// Number-keyed buckets without a lag: deleted across the full window.
	for i := range endExclusive {
		assert.False(t, blockCommitmentsExist(database, i),
			"commitments at block %d should be deleted", i)
		assert.False(t, stateUpdateExists(database, i),
			"state update at block %d should be deleted", i)
		assert.False(t, transactionsExist(database, i),
			"transactions at block %d should be deleted", i)
	}
	for i := endExclusive; i < totalBlocks; i++ {
		assert.True(t, blockCommitmentsExist(database, i),
			"commitments at block %d should be untouched", i)
		assert.True(t, stateUpdateExists(database, i),
			"state update at block %d should be untouched", i)
		assert.True(t, transactionsExist(database, i),
			"transactions at block %d should be untouched", i)
	}
}
