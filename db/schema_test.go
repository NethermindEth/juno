package db_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryLastUpdateBlock(t *testing.T) {
	addr := felt.NewFromUint64[felt.Felt](1)
	loc := felt.NewFromUint64[felt.Felt](2)
	prefix := db.ContractStorageHistoryKey(addr, loc)

	putAt := func(t *testing.T, database db.KeyValueStore, blockNum uint64) {
		t.Helper()
		key := db.ContractStorageHistoryAtBlockKey(addr, loc, blockNum)
		require.NoError(t, database.Put(key, []byte("value")))
	}

	t.Run("empty db returns not found", func(t *testing.T) {
		database := memory.New()
		defer database.Close()

		blockNum, found, err := db.HistoryLastUpdateBlock(database, prefix, 10)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Equal(t, uint64(0), blockNum)
	})

	t.Run("exact match returns upToBlock", func(t *testing.T) {
		database := memory.New()
		defer database.Close()

		putAt(t, database, 5)

		blockNum, found, err := db.HistoryLastUpdateBlock(database, prefix, 5)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, uint64(5), blockNum)
	})

	t.Run("returns latest block before upToBlock", func(t *testing.T) {
		database := memory.New()
		defer database.Close()

		putAt(t, database, 3)
		putAt(t, database, 7)
		putAt(t, database, 10)

		blockNum, found, err := db.HistoryLastUpdateBlock(database, prefix, 8)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, uint64(7), blockNum)
	})

	t.Run("all entries are after upToBlock returns not found", func(t *testing.T) {
		database := memory.New()
		defer database.Close()

		putAt(t, database, 5)
		putAt(t, database, 10)

		blockNum, found, err := db.HistoryLastUpdateBlock(database, prefix, 3)
		require.NoError(t, err)
		assert.False(t, found)
		assert.Equal(t, uint64(0), blockNum)
	})

	t.Run("multiple entries with exact match at upToBlock", func(t *testing.T) {
		database := memory.New()
		defer database.Close()

		putAt(t, database, 1)
		putAt(t, database, 5)
		putAt(t, database, 9)

		blockNum, found, err := db.HistoryLastUpdateBlock(database, prefix, 9)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, uint64(9), blockNum)
	})

	t.Run("different contract prefix does not affect result", func(t *testing.T) {
		database := memory.New()
		defer database.Close()

		otherAddr := new(felt.Felt).SetUint64(99)
		otherPrefix := db.ContractStorageHistoryKey(otherAddr, loc)

		putAt(t, database, 5)

		// Write entries under a different prefix
		otherKey := db.ContractStorageHistoryAtBlockKey(otherAddr, loc, 3)
		require.NoError(t, database.Put(otherKey, []byte("other")))

		blockNum, found, err := db.HistoryLastUpdateBlock(database, prefix, 10)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, uint64(5), blockNum)

		// Query the other prefix
		blockNum, found, err = db.HistoryLastUpdateBlock(database, otherPrefix, 10)
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, uint64(3), blockNum)
	})
}
