package pruner_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/pruner/testutils"
	_ "github.com/NethermindEth/juno/utils/cbor/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockPrunedError(t *testing.T) {
	t.Run("Is matches the ErrBlockPruned sentinel", func(t *testing.T) {
		e := &pruner.BlockPrunedError{BlockNumber: 5, OldestRetained: 10}
		assert.True(t, errors.Is(e, pruner.ErrBlockPruned))
		assert.False(t, errors.Is(e, db.ErrKeyNotFound))
	})

	t.Run("As surfaces the block-number / oldest-retained fields", func(t *testing.T) {
		var target *pruner.BlockPrunedError
		err := error(&pruner.BlockPrunedError{BlockNumber: 5, OldestRetained: 10})
		require.True(t, errors.As(err, &target))
		assert.Equal(t, uint64(5), target.BlockNumber)
		assert.Equal(t, uint64(10), target.OldestRetained)
	})

	t.Run("message format depends on whether OldestRetained is known", func(t *testing.T) {
		// OldestRetained=0 means the lookup itself failed (e.g. empty DB),
		// so the message must not claim a specific oldest block.
		unknown := &pruner.BlockPrunedError{BlockNumber: 5}
		assert.Contains(t, unknown.Error(), "below the node's retention floor")
		assert.NotContains(t, unknown.Error(), "oldest retained")

		known := &pruner.BlockPrunedError{BlockNumber: 5, OldestRetained: 10}
		assert.Contains(t, known.Error(), "oldest retained block is 10")
	})
}

func TestRequireRetained(t *testing.T) {
	t.Run("retained block returns nil", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range uint64(5) {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, pruner.RequireRetained(database, 3))
	})

	t.Run("pruned block surfaces oldest retained", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range uint64(5) {
			testutils.StoreBlock(t, database, i)
		}
		// Drop the commitment for block 0 — RequireRetained probes the
		// commitment bucket as the source of truth for "still retained".
		testutils.WithBatch(t, database, func(batch db.Batch) error {
			return batch.Delete(db.BlockCommitmentsKey(0))
		})

		err := pruner.RequireRetained(database, 0)
		require.Error(t, err)
		assert.True(t, errors.Is(err, pruner.ErrBlockPruned))

		var bpe *pruner.BlockPrunedError
		require.True(t, errors.As(err, &bpe))
		assert.Equal(t, uint64(0), bpe.BlockNumber)
		assert.Equal(t, uint64(1), bpe.OldestRetained,
			"OldestRetainedBlock should report block 1 once block 0's commitment is gone")
	})

	t.Run("empty DB reports unknown oldest", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		err := pruner.RequireRetained(database, 5)
		require.Error(t, err)

		var bpe *pruner.BlockPrunedError
		require.True(t, errors.As(err, &bpe))
		assert.Equal(t, uint64(5), bpe.BlockNumber)
		assert.Zero(t, bpe.OldestRetained,
			"OldestRetained=0 signals the lookup itself failed, not block 0")
	})
}

func TestRequireStateRetainedByBlockNumber(t *testing.T) {
	t.Run("unseeded floor allows retained state", func(t *testing.T) {
		const blockNumber = uint64(3)

		database := memory.New()
		for i := range uint64(5) {
			testutils.StoreBlock(t, database, i)
		}
		floor := &pruner.RetentionFloor{}
		require.NoError(t, pruner.RequireStateRetainedByBlockNumber(database, floor, blockNumber))
	})

	t.Run("unseeded floor rejects pruned state when header remains", func(t *testing.T) {
		const blockNumber = uint64(1)

		database := memory.New()
		blocks := make([]*testutils.StoredBlock, 5)
		for i := range uint64(5) {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		prunedHash := blocks[blockNumber].Header.Hash
		require.NoError(t, database.Delete(db.BlockHeaderNumbersByHashKey(prunedHash)))

		floor := &pruner.RetentionFloor{}
		err := pruner.RequireStateRetainedByBlockNumber(database, floor, blockNumber)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("seeded floor skips the header probe", func(t *testing.T) {
		const blockNumber = uint64(1)

		database := memory.New()
		blocks := make([]*testutils.StoredBlock, 5)
		for i := range uint64(5) {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, 4))

		floor, err := pruner.NewRetentionFloor(database)
		require.NoError(t, err)

		require.NoError(t,
			database.Delete(db.BlockHeaderNumbersByHashKey(blocks[blockNumber].Header.Hash)))
		require.NoError(t, pruner.RequireStateRetainedByBlockNumber(database, floor, blockNumber))
	})

	t.Run("seeded floor rejects blocks below the floor", func(t *testing.T) {
		database := memory.New()
		for i := uint64(5); i <= 7; i++ {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, 7))

		floor, err := pruner.NewRetentionFloor(database)
		require.NoError(t, err)

		err = pruner.RequireStateRetainedByBlockNumber(database, floor, 3)
		require.ErrorIs(t, err, db.ErrKeyNotFound)

		require.NoError(t, pruner.RequireStateRetainedByBlockNumber(database, floor, 4))
		require.NoError(t, pruner.RequireStateRetainedByBlockNumber(database, floor, 5))
		require.NoError(t, pruner.RequireStateRetainedByBlockNumber(database, floor, 7))
	})

	t.Run("seeded floor rejects blocks above the chain height", func(t *testing.T) {
		database := memory.New()
		for i := range uint64(5) {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, 4))

		floor, err := pruner.NewRetentionFloor(database)
		require.NoError(t, err)

		err = pruner.RequireStateRetainedByBlockNumber(database, floor, 5)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestRetentionFloorSeed(t *testing.T) {
	t.Run("reseeding after out-of-band pruning raises the floor", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range uint64(10) {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, 9))

		floor, err := pruner.NewRetentionFloor(database)
		require.NoError(t, err)

		batch := database.NewBatch()
		require.NoError(t, pruner.PruneBlockDataUpto(batch, 5))
		require.NoError(t, batch.Write())

		require.NoError(t, floor.Seed(database))

		err = pruner.RequireStateRetainedByBlockNumber(database, floor, 3)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
		require.NoError(t, pruner.RequireStateRetainedByBlockNumber(database, floor, 4))
	})

	t.Run("seeding an empty database leaves nothing rejected", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		floor := &pruner.RetentionFloor{}
		require.NoError(t, floor.Seed(database))

		for i := range uint64(3) {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, 2))
		require.NoError(t, pruner.RequireStateRetainedByBlockNumber(database, floor, 0))
	})
}

func TestNewRetentionFloor(t *testing.T) {
	t.Run("empty database retains every block stored later", func(t *testing.T) {
		database := memory.New()
		floor, err := pruner.NewRetentionFloor(database)
		require.NoError(t, err)

		for i := range uint64(3) {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, 2))

		require.NoError(t, pruner.RequireStateRetainedByBlockNumber(database, floor, 0))
		require.NoError(t, pruner.RequireStateRetainedByBlockNumber(database, floor, 2))
		err = pruner.RequireStateRetainedByBlockNumber(database, floor, 3)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestStateRootIfStateRetainedByBlockNumber(t *testing.T) {
	t.Run("returns the state root for retained state", func(t *testing.T) {
		const blockNumber = uint64(3)

		database := memory.New()
		blocks := make([]*testutils.StoredBlock, 5)
		for i := range uint64(5) {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}

		floor := &pruner.RetentionFloor{}
		stateRoot, err := pruner.StateRootIfStateRetainedByBlockNumber(database, floor, blockNumber)
		require.NoError(t, err)
		assert.Equal(t, blocks[blockNumber].Header.GlobalStateRoot, stateRoot)
	})

	t.Run("unseeded floor rejects pruned state when header remains", func(t *testing.T) {
		const blockNumber = uint64(1)

		database := memory.New()
		blocks := make([]*testutils.StoredBlock, 5)
		for i := range uint64(5) {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		prunedHash := blocks[blockNumber].Header.Hash
		require.NoError(t, database.Delete(db.BlockHeaderNumbersByHashKey(prunedHash)))

		floor := &pruner.RetentionFloor{}
		_, err := pruner.StateRootIfStateRetainedByBlockNumber(database, floor, blockNumber)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("seeded floor skips the hash probe", func(t *testing.T) {
		const blockNumber = uint64(1)

		database := memory.New()
		blocks := make([]*testutils.StoredBlock, 5)
		for i := range uint64(5) {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}

		floor, err := pruner.NewRetentionFloor(database)
		require.NoError(t, err)

		require.NoError(t,
			database.Delete(db.BlockHeaderNumbersByHashKey(blocks[blockNumber].Header.Hash)))
		stateRoot, err := pruner.StateRootIfStateRetainedByBlockNumber(database, floor, blockNumber)
		require.NoError(t, err)
		assert.Equal(t, blocks[blockNumber].Header.GlobalStateRoot, stateRoot)
	})

	t.Run("seeded floor rejects blocks below the floor", func(t *testing.T) {
		database := memory.New()
		for i := uint64(5); i <= 7; i++ {
			testutils.StoreBlock(t, database, i)
		}

		floor, err := pruner.NewRetentionFloor(database)
		require.NoError(t, err)

		_, err = pruner.StateRootIfStateRetainedByBlockNumber(database, floor, 3)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("missing block returns ErrKeyNotFound", func(t *testing.T) {
		database := memory.New()
		floor := &pruner.RetentionFloor{}
		_, err := pruner.StateRootIfStateRetainedByBlockNumber(database, floor, 0)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestOldestRetainedBlock(t *testing.T) {
	t.Run("empty database returns ErrKeyNotFound", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)

		_, err := pruner.OldestRetainedBlock(database)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("returns lowest block number with commitments", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)

		for i := uint64(5); i <= 7; i++ {
			testutils.StoreBlock(t, database, i)
		}

		num, err := pruner.OldestRetainedBlock(database)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), num)
	})
}
