package core

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/bits-and-blooms/bitset"
	"github.com/stretchr/testify/assert"
)

func TestTrieTxn(t *testing.T) {
	testDb := pebble.NewMemTest()
	prefix := []byte{37, 44}

	key := bitset.New(44)
	node := new(trie.Node)
	node.Value, _ = new(felt.Felt).SetRandom()

	// put a node
	assert.NoError(t, testDb.Update(func(txn db.Transaction) error {
		tTxn := &TransactionStorage{txn, prefix}

		return tTxn.Put(key, node)
	}))

	// get node
	assert.NoError(t, testDb.View(func(txn db.Transaction) error {
		tTxn := &TransactionStorage{txn, prefix}

		got, err := tTxn.Get(key)
		assert.NoError(t, err)
		assert.Equal(t, true, got.Equal(node))

		return err
	}))

	// in case of an error, tx should roll back
	assert.Error(t, testDb.Update(func(txn db.Transaction) error {
		tTxn := &TransactionStorage{txn, prefix}

		if err := tTxn.Delete(key); err != nil {
			t.Error(err)
		}

		return errors.New("should rollback")
	}))

	// should still be able to get the node
	assert.NoError(t, testDb.View(func(txn db.Transaction) error {
		tTxn := &TransactionStorage{txn, prefix}

		got, err := tTxn.Get(key)
		assert.Equal(t, true, got.Equal(node))

		return err
	}))

	// successful delete
	assert.NoError(t, testDb.Update(func(txn db.Transaction) error {
		tTxn := &TransactionStorage{txn, prefix}
		return tTxn.Delete(key)
	}))

	// should error with key not found
	assert.EqualError(t, testDb.View(func(txn db.Transaction) error {
		tTxn := &TransactionStorage{txn, prefix}
		_, err := tTxn.Get(key)
		return err
	}), db.ErrKeyNotFound.Error())
}
