package pruner_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/stretchr/testify/assert"
)

// TestPruneBlockDataUpto exercises the range-tombstone half on its own,
// confirming the BlockHashLag carve-out for headers without the per-block
// hash-keyed work that pruneUpto wraps around it.
func TestPruneBlockDataUpto(t *testing.T) {
	const totalBlocks uint64 = 30
	const endExclusive uint64 = 20
	lag := core.BlockHashLag

	database := testutils.NewPebbleTestDB(t)
	for i := range totalBlocks {
		testutils.StoreBlock(t, database, i)
	}

	testutils.WithBatch(t, database, func(batch db.Batch) error {
		return pruner.PruneBlockDataUpto(batch, endExclusive)
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
