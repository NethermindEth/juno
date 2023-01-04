package core

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
	"github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/assert"
)

func newTestDb() *badger.DB {
	opt := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opt)
	if err != nil {
		panic(err)
	}
	return db
}

func TestTrieTxn(t *testing.T) {
	db := newTestDb()
	prefix := []byte{37, 44}

	key := bitset.New(44)
	node := new(TrieNode)

	// put a node
	if err := db.Update(func(txn *badger.Txn) error {
		tTxn := &TrieTxn{txn, prefix}

		value, _ := new(felt.Felt).SetRandom()
		node.UnmarshalBinary(value.Marshal())

		return tTxn.Put(key, node)
	}); err != nil {
		t.Error(err)
	}

	// get node
	if err := db.View(func(txn *badger.Txn) error {
		tTxn := &TrieTxn{txn, prefix}

		got, err := tTxn.Get(key)
		assert.Equal(t, true, got.Equal(node))

		return err
	}); err != nil {
		t.Error(err)
	}

	// in case of an error, tx should roll back
	if err := db.Update(func(txn *badger.Txn) error {
		tTxn := &TrieTxn{txn, prefix}

		if err := tTxn.Delete(key); err != nil {
			t.Error(err)
		}

		return errors.New("should rollback")
	}); err == nil {
		t.Error("Should've errored")
	}

	// should still be able to get the node
	if err := db.View(func(txn *badger.Txn) error {
		tTxn := &TrieTxn{txn, prefix}

		got, err := tTxn.Get(key)
		assert.Equal(t, true, got.Equal(node))

		return err
	}); err != nil {
		t.Error(err)
	}

	// successful delete
	if err := db.Update(func(txn *badger.Txn) error {
		tTxn := &TrieTxn{txn, prefix}
		return tTxn.Delete(key)
	}); err != nil {
		t.Error(err)
	}

	// should error with key not found
	assert.EqualError(t, db.View(func(txn *badger.Txn) error {
		tTxn := &TrieTxn{txn, prefix}
		_, err := tTxn.Get(key)
		return err
	}), "Key not found")
}
