package db

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTransaction(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	txn := db.NewTransaction(true)
	err := txn.Set([]byte("key"), []byte("value"))
	assert.Nil(t, err)
	err = txn.Commit()
	assert.Nil(t, err)

	readOnlyTxn := db.NewTransaction(false)
	val, err := readOnlyTxn.Get([]byte("key"))
	assert.Nil(t, err)
	assert.Equal(t, "value", string(val))
}

func TestDiscardTransaction(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	txn := db.NewTransaction(true)
	err := txn.Set([]byte("key"), []byte("value"))
	assert.Nil(t, err)
	txn.Discard()

	readOnlyTxn := db.NewTransaction(false)
	_, err = readOnlyTxn.Get([]byte("key"))
	assert.Equal(t, ErrKeyNotFound, err)
}

func TestConcurrentTransactions(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	txn1 := db.NewTransaction(true)
	txn2 := db.NewTransaction(true)

	txn1.Set([]byte("key1"), []byte("value1"))
	_, err := txn2.Get([]byte("key1"))
	assert.Equal(t, ErrKeyNotFound, err)

	txn2.Set([]byte("key2"), []byte("value2"))
	_, err = txn1.Get([]byte("key2"))
	assert.Equal(t, ErrKeyNotFound, err)
}

func TestViewUpdate(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	// Test View
	err := db.View(func(txn Transaction) error {
		val, err := txn.Get([]byte("key"))
		if err == ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		assert.Equal(t, "", string(val))
		return nil
	})
	assert.Nil(t, err)

	// Test Update
	err = db.Update(func(txn Transaction) error {
		return txn.Set([]byte("key"), []byte("value"))
	})
	assert.Nil(t, err)

	// Check value
	err = db.View(func(txn Transaction) error {
		val, err := txn.Get([]byte("key"))
		if err != nil {
			return err
		}
		assert.Equal(t, "value", string(val))
		return nil
	})
	assert.Nil(t, err)
}

func TestUpdateDiscardOnError(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	// Test Update
	err := db.Update(func(txn Transaction) error {
		err := txn.Set([]byte("key"), []byte("value"))
		assert.Nil(t, err)
		return fmt.Errorf("error")
	})
	assert.NotNil(t, err)

	// Check key is not in the db
	err = db.View(func(txn Transaction) error {
		_, err := txn.Get([]byte("key"))
		assert.Equal(t, ErrKeyNotFound, err)
		return nil
	})
}

func TestDiscardCommit(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	txn := db.NewTransaction(true)
	err := txn.Set([]byte("key"), []byte("value"))
	assert.Nil(t, err)
	txn.Discard()

	err = txn.Commit()
	assert.NotNil(t, err, "discarded transaction should not be able to commit")
}

func TestZeroLengthValue(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	err := db.Update(func(txn Transaction) error {
		err := txn.Set([]byte("key"), []byte{})
		assert.Nil(t, err, "setting a key with a zero-length value should be allowed")
		val, err := txn.Get([]byte("key"))
		assert.Nil(t, err)
		assert.Equal(t, []byte{}, val)
		return nil
	})
	assert.Nil(t, err)
}

func TestZeroLengthKey(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	err := db.Update(func(txn Transaction) error {
		err := txn.Set([]byte{}, []byte("value"))
		assert.NotNil(t, err, "setting a key with a zero-length key should not be allowed")
		return nil
	})
	assert.Nil(t, err)
}

func TestNilKey(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	err := db.Update(func(txn Transaction) error {
		err := txn.Set(nil, []byte("value"))
		assert.NotNil(t, err, "setting a key with a nil key should not be allowed")
		return nil
	})
	assert.Nil(t, err)
}

func TestNilValue(t *testing.T) {
	db := NewTestDb()
	defer db.Close()

	err := db.Update(func(txn Transaction) error {
		err := txn.Set([]byte("key"), nil)
		assert.Nil(t, err, "setting a key with a nil value should be allowed")
		val, err := txn.Get([]byte("key"))
		assert.Nil(t, err)
		assert.Equal(t, val, []byte{})
		return nil
	})
	assert.Nil(t, err)
}
