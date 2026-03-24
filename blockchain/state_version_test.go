package blockchain_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

func TestValidateStateVersion(t *testing.T) {
	t.Run("fresh database is always accepted", func(t *testing.T) {
		testDB := memory.New()
		require.NoError(t, blockchain.ValidateStateVersion(testDB, false))
		require.NoError(t, blockchain.ValidateStateVersion(testDB, true))
	})

	t.Run("deprecated-state DB with new-state flag returns error", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ClassesTrie)

		err := blockchain.ValidateStateVersion(testDB, true)
		require.ErrorContains(t, err, "deprecated state")
		require.ErrorContains(t, err, "--new-state")
	})

	t.Run("new-state DB without new-state flag returns error", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ClassTrie)

		err := blockchain.ValidateStateVersion(testDB, false)
		require.ErrorContains(t, err, "new state")
		require.ErrorContains(t, err, "--new-state")
	})

	t.Run("deprecated-state DB without new-state flag is accepted", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ClassesTrie)

		require.NoError(t, blockchain.ValidateStateVersion(testDB, false))
	})

	t.Run("new-state DB with new-state flag is accepted", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)
		seedBucket(t, testDB, db.ClassTrie)

		require.NoError(t, blockchain.ValidateStateVersion(testDB, true))
	})

	t.Run("DB with blocks but no trie data is accepted regardless of flag", func(t *testing.T) {
		testDB := memory.New()
		seedChainHeight(t, testDB)

		require.NoError(t, blockchain.ValidateStateVersion(testDB, false))
		require.NoError(t, blockchain.ValidateStateVersion(testDB, true))
	})
}

func seedChainHeight(t *testing.T, testDB db.KeyValueStore) {
	t.Helper()
	batch := testDB.NewBatch()
	require.NoError(t, core.WriteChainHeight(batch, 1))
	require.NoError(t, batch.Write())
}

func seedBucket(t *testing.T, testDB db.KeyValueStore, bucket db.Bucket) {
	t.Helper()
	batch := testDB.NewBatch()
	key := append(bucket.Key(), []byte("sentinel")...)
	require.NoError(t, batch.Put(key, []byte("1")))
	require.NoError(t, batch.Write())
}
