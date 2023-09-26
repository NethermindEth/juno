package trie_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionStorage(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	prefix := []byte{37, 44}
	key := trie.NewKey(44, nil)

	value, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	node := &trie.Node{
		Value: value,
	}

	t.Run("put a node", func(t *testing.T) {
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			tTxn := trie.NewTransactionStorage(txn, prefix)
			return tTxn.Put(&key, node)
		}))
	})

	t.Run("get a node", func(t *testing.T) {
		require.NoError(t, testDB.View(func(txn db.Transaction) error {
			tTxn := trie.NewTransactionStorage(txn, prefix)
			var got *trie.Node
			got, err = tTxn.Get(&key)
			require.NoError(t, err)
			assert.Equal(t, node, got)
			return err
		}))
	})

	t.Run("roll back on error", func(t *testing.T) {
		// Successfully delete a node and return an error to force a roll back.
		require.Error(t, testDB.Update(func(txn db.Transaction) error {
			tTxn := trie.NewTransactionStorage(txn, prefix)
			err = tTxn.Delete(&key)
			require.NoError(t, err)
			return errors.New("should rollback")
		}))

		// If the transaction was properly rolled back, the node that we
		// "deleted" should still exist in the db.
		require.NoError(t, testDB.View(func(txn db.Transaction) error {
			tTxn := trie.NewTransactionStorage(txn, prefix)
			var got *trie.Node
			got, err = tTxn.Get(&key)
			assert.Equal(t, node, got)
			return err
		}))
	})

	t.Run("delete a node", func(t *testing.T) {
		// Delete a node.
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			tTxn := trie.NewTransactionStorage(txn, prefix)
			return tTxn.Delete(&key)
		}))

		// Node should no longer exist in the database.
		require.EqualError(t, testDB.View(func(txn db.Transaction) error {
			tTxn := trie.NewTransactionStorage(txn, prefix)
			_, err = tTxn.Get(&key)
			return err
		}), db.ErrKeyNotFound.Error())
	})

	rootKey := trie.NewKey(8, []byte{0x2})

	t.Run("put root key", func(t *testing.T) {
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			tTxn := trie.NewTransactionStorage(txn, prefix)
			return tTxn.PutRootKey(&rootKey)
		}))
	})

	t.Run("read root key", func(t *testing.T) {
		require.NoError(t, testDB.View(func(txn db.Transaction) error {
			tTxn := trie.NewTransactionStorage(txn, prefix)
			gotRootKey, err := tTxn.RootKey()
			require.NoError(t, err)
			assert.Equal(t, rootKey, *gotRootKey)
			return nil
		}))
	})

	t.Run("delete root key", func(t *testing.T) {
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			tTxn := trie.NewTransactionStorage(txn, prefix)
			require.NoError(t, tTxn.DeleteRootKey())
			_, err := tTxn.RootKey()
			require.ErrorIs(t, err, db.ErrKeyNotFound)
			return nil
		}))
	})
}
