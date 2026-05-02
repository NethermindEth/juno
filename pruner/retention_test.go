package pruner_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/pruner/testutils"
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
		database := testutils.NewTestDB(t)
		for i := range uint64(5) {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, pruner.RequireRetained(database, 3))
	})

	t.Run("pruned block surfaces oldest retained", func(t *testing.T) {
		database := testutils.NewTestDB(t)
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
		database := testutils.NewTestDB(t)
		err := pruner.RequireRetained(database, 5)
		require.Error(t, err)

		var bpe *pruner.BlockPrunedError
		require.True(t, errors.As(err, &bpe))
		assert.Equal(t, uint64(5), bpe.BlockNumber)
		assert.Zero(t, bpe.OldestRetained,
			"OldestRetained=0 signals the lookup itself failed, not block 0")
	})
}

func TestHeaderByNumberIfStateRetained(t *testing.T) {
	t.Run("fully retained block returns the header", func(t *testing.T) {
		database := testutils.NewTestDB(t)
		blocks := make([]*testutils.StoredBlock, 5)
		for i := range uint64(5) {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		header, err := pruner.HeaderByNumberIfStateRetained(database, 3)
		require.NoError(t, err)
		assert.Equal(t, blocks[3].Header.Hash, header.Hash)
	})

	t.Run("block with header but no hash→number is treated as state-pruned", func(t *testing.T) {
		// This mirrors the lag-window state of pruned blocks: header kept,
		// hash→number swept. State accessors must NOT serve these.
		database := testutils.NewTestDB(t)
		blocks := make([]*testutils.StoredBlock, 5)
		for i := range uint64(5) {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		testutils.WithBatch(t, database, func(batch db.Batch) error {
			return batch.Delete(db.BlockHeaderNumbersByHashKey(blocks[1].Header.Hash))
		})

		_, err := pruner.HeaderByNumberIfStateRetained(database, 1)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("missing block returns ErrKeyNotFound", func(t *testing.T) {
		database := testutils.NewTestDB(t)
		_, err := pruner.HeaderByNumberIfStateRetained(database, 0)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}

func TestHeaderByHashIfStateRetained(t *testing.T) {
	t.Run("fully retained block returns the header", func(t *testing.T) {
		database := testutils.NewTestDB(t)
		blocks := make([]*testutils.StoredBlock, 5)
		for i := range uint64(5) {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		header, err := pruner.HeaderByHashIfStateRetained(database, blocks[2].Header.Hash)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), header.Number)
	})

	t.Run("missing hash returns ErrKeyNotFound", func(t *testing.T) {
		database := testutils.NewTestDB(t)
		unknownHash := felt.NewRandom[felt.Felt]()
		_, err := pruner.HeaderByHashIfStateRetained(database, unknownHash)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("block whose hash→number was deleted is unreachable by hash", func(t *testing.T) {
		database := testutils.NewTestDB(t)
		blocks := make([]*testutils.StoredBlock, 5)
		for i := range uint64(5) {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		testutils.WithBatch(t, database, func(batch db.Batch) error {
			return batch.Delete(db.BlockHeaderNumbersByHashKey(blocks[1].Header.Hash))
		})
		_, err := pruner.HeaderByHashIfStateRetained(database, blocks[1].Header.Hash)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}
