package trie_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorage(t *testing.T) {
	memDB := memory.New()
	txn := memDB.NewSnapshotBatch()
	prefix := []byte{37, 44}
	key := trie.NewBitArray(44, 0)

	value := felt.NewRandom[felt.Felt]()

	node := &trie.Node{
		Value: value,
	}

	t.Run("put a node", func(t *testing.T) {
		storage := trie.NewStorage(txn, prefix)
		require.NoError(t, storage.Put(&key, node))
	})

	t.Run("get a node", func(t *testing.T) {
		storage := trie.NewStorage(txn, prefix)
		got, err := storage.Get(&key)
		require.NoError(t, err)
		assert.Equal(t, node, got)
	})

	t.Run("delete a node", func(t *testing.T) {
		// Delete a node.
		storage := trie.NewStorage(txn, prefix)
		require.NoError(t, storage.Delete(&key))

		// Node should no longer exist in the database.
		_, err := storage.Get(&key)
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	rootKey := trie.NewBitArray(8, 2)

	t.Run("put root key", func(t *testing.T) {
		storage := trie.NewStorage(txn, prefix)
		require.NoError(t, storage.PutRootKey(&rootKey))
	})

	t.Run("read root key", func(t *testing.T) {
		storage := trie.NewStorage(txn, prefix)
		gotRootKey, err := storage.RootKey()
		require.NoError(t, err)
		assert.Equal(t, &rootKey, gotRootKey)
	})

	t.Run("delete root key", func(t *testing.T) {
		storage := trie.NewStorage(txn, prefix)
		require.NoError(t, storage.DeleteRootKey())
		_, err := storage.RootKey()
		require.ErrorIs(t, err, db.ErrKeyNotFound)
	})
}
