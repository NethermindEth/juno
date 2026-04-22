package core

import (
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

func TestValidateStateVersion(t *testing.T) {
	t.Run("fresh database is always accepted", func(t *testing.T) {
		testDB := memory.New()
		require.NoError(t, ValidateStateVersion(testDB, false))
		require.NoError(t, ValidateStateVersion(testDB, true))
	})

	t.Run("old-state DB with new-state flag returns error", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ClassesTrie)

		err := ValidateStateVersion(testDB, true)
		require.ErrorContains(t, err, "existing state")
		require.ErrorContains(t, err, "--new-state")
	})

	t.Run("new-state DB without new-state flag returns error", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ClassTrie)

		err := ValidateStateVersion(testDB, false)
		require.ErrorContains(t, err, "new state")
		require.ErrorContains(t, err, "--new-state")
	})

	t.Run("old-state DB without new-state flag is accepted", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ClassesTrie)

		require.NoError(t, ValidateStateVersion(testDB, false))
	})

	t.Run("new-state DB with new-state flag is accepted", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ClassTrie)

		require.NoError(t, ValidateStateVersion(testDB, true))
	})

	t.Run("DB with blocks but no trie data is accepted regardless of flag", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)

		require.NoError(t, ValidateStateVersion(testDB, false))
		require.NoError(t, ValidateStateVersion(testDB, true))
	})

	t.Run("old-state detected via StateTrie when ClassesTrie is empty", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.StateTrie)

		err := ValidateStateVersion(testDB, true)
		require.ErrorContains(t, err, "existing state")
		require.NoError(t, ValidateStateVersion(testDB, false))
	})

	t.Run("old-state detected via ContractStorage when other old buckets empty", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ContractStorage)

		err := ValidateStateVersion(testDB, true)
		require.ErrorContains(t, err, "existing state")
		require.NoError(t, ValidateStateVersion(testDB, false))
	})

	t.Run("new-state detected via ContractTrieContract when ClassTrie is empty", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ContractTrieContract)

		err := ValidateStateVersion(testDB, false)
		require.ErrorContains(t, err, "new state")
		require.NoError(t, ValidateStateVersion(testDB, true))
	})

	t.Run("new-state detected via ContractTrieStorage when ClassTrie is empty", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ContractTrieStorage)

		err := ValidateStateVersion(testDB, false)
		require.ErrorContains(t, err, "new state")
		require.NoError(t, ValidateStateVersion(testDB, true))
	})
}

func seedChainHeight(t *testing.T, testDB db.KeyValueStore) {
	t.Helper()
	batch := testDB.NewBatch()
	require.NoError(t, WriteChainHeight(batch, 1))
	require.NoError(t, batch.Write())
}

func seedBucket(t *testing.T, testDB db.KeyValueStore, bucket db.Bucket) {
	t.Helper()
	batch := testDB.NewBatch()
	key := append(bucket.Key(), []byte("sentinel")...)
	require.NoError(t, batch.Put(key, []byte("1")))
	require.NoError(t, batch.Write())
}
