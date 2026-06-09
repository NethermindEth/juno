package pruner

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
