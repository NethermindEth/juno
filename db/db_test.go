package db_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
)

var noop = func(val []byte) error {
	return nil
}

func TestNewTransaction(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	txn := testDb.NewTransaction(true)
	err := txn.Set([]byte("key"), []byte("value"))
	assert.Nil(t, err)
	err = txn.Commit()
	assert.Nil(t, err)

	readOnlyTxn := testDb.NewTransaction(false)
	err = readOnlyTxn.Get([]byte("key"), func(val []byte) error {
		assert.Equal(t, "value", string(val))
		return nil
	})
	assert.Nil(t, err)
}

func TestDiscardTransaction(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	txn := testDb.NewTransaction(true)
	err := txn.Set([]byte("key"), []byte("value"))
	assert.Nil(t, err)
	txn.Discard()

	readOnlyTxn := testDb.NewTransaction(false)
	err = readOnlyTxn.Get([]byte("key"), noop)
	assert.Equal(t, db.ErrKeyNotFound, err)
}

func TestConcurrentTransactions(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	txn1 := testDb.NewTransaction(true)
	txn2 := testDb.NewTransaction(true)

	txn1.Set([]byte("key1"), []byte("value1"))
	err := txn2.Get([]byte("key1"), noop)
	assert.Equal(t, db.ErrKeyNotFound, err)

	txn2.Set([]byte("key2"), []byte("value2"))
	err = txn1.Get([]byte("key2"), noop)
	assert.Equal(t, db.ErrKeyNotFound, err)
}

func TestViewUpdate(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	// Test View
	err := testDb.View(func(txn db.Transaction) error {
		err := txn.Get([]byte("key"), func(val []byte) error {
			assert.Equal(t, "", string(val))
			return nil
		})
		if err == db.ErrKeyNotFound {
			return nil
		}
		return err
	})
	assert.Nil(t, err)

	// Test Update
	err = testDb.Update(func(txn db.Transaction) error {
		return txn.Set([]byte("key"), []byte("value"))
	})
	assert.Nil(t, err)

	// Check value
	err = testDb.View(func(txn db.Transaction) error {
		err = txn.Get([]byte("key"), func(val []byte) error {
			assert.Equal(t, "value", string(val))
			return nil
		})
		return err
	})
	assert.Nil(t, err)
}

func TestUpdateDiscardOnError(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	// Test Update
	err := testDb.Update(func(txn db.Transaction) error {
		err := txn.Set([]byte("key"), []byte("value"))
		assert.Nil(t, err)
		return fmt.Errorf("error")
	})
	assert.NotNil(t, err)

	// Check key is not in the db
	err = testDb.View(func(txn db.Transaction) error {
		err = txn.Get([]byte("key"), noop)
		assert.Equal(t, db.ErrKeyNotFound, err)
		return nil
	})
}

func TestDiscardCommit(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	txn := testDb.NewTransaction(true)
	err := txn.Set([]byte("key"), []byte("value"))
	assert.Nil(t, err)
	txn.Discard()

	err = txn.Commit()
	assert.NotNil(t, err, "discarded transaction should not be able to commit")
}

func TestZeroLengthValue(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	err := testDb.Update(func(txn db.Transaction) error {
		err := txn.Set([]byte("key"), []byte{})
		assert.Nil(t, err, "setting a key with a zero-length value should be allowed")
		err = txn.Get([]byte("key"), func(val []byte) error {
			assert.Equal(t, []byte{}, val)
			return nil
		})
		return err
	})
	assert.Nil(t, err)
}

func TestZeroLengthKey(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	err := testDb.Update(func(txn db.Transaction) error {
		err := txn.Set([]byte{}, []byte("value"))
		assert.NotNil(t, err, "setting a key with a zero-length key should not be allowed")
		return nil
	})
	assert.Nil(t, err)
}

func TestNilKey(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	err := testDb.Update(func(txn db.Transaction) error {
		err := txn.Set(nil, []byte("value"))
		assert.NotNil(t, err, "setting a key with a nil key should not be allowed")
		return nil
	})
	assert.Nil(t, err)
}

func TestNilValue(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	err := testDb.Update(func(txn db.Transaction) error {
		err := txn.Set([]byte("key"), nil)
		assert.Nil(t, err, "setting a key with a nil value should be allowed")
		err = txn.Get([]byte("key"), func(val []byte) error {
			assert.Nil(t, val)
			return nil
		})
		return err
	})
	assert.Nil(t, err)
}

func TestSeek(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()

	err := testDb.Update(func(txn db.Transaction) error {
		err := txn.Set([]byte{1}, []byte{1})
		assert.NoError(t, err)
		err = txn.Set([]byte{3}, []byte{3})
		assert.NoError(t, err)
		return nil
	})
	assert.NoError(t, err)

	testDb.View(func(txn db.Transaction) error {
		err := txn.Seek([]byte{0}, func(next *db.Entry) error {
			assert.Equal(t, []byte{1}, next.Key)
			assert.Equal(t, []byte{1}, next.Value)
			return nil
		})
		assert.NoError(t, err)

		err = txn.Seek([]byte{2}, func(next *db.Entry) error {
			assert.Equal(t, []byte{3}, next.Key)
			assert.Equal(t, []byte{3}, next.Value)
			return nil
		})
		assert.NoError(t, err)

		err = txn.Seek([]byte{3}, func(next *db.Entry) error {
			assert.Equal(t, []byte{3}, next.Key)
			assert.Equal(t, []byte{3}, next.Value)
			return nil
		})
		assert.NoError(t, err)

		err = txn.Seek([]byte{4}, func(next *db.Entry) error {
			assert.Nil(t, next)
			return nil
		})
		assert.NoError(t, err)
		return nil
	})
}
