package db_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	txn2 := testDb.NewTransaction(false)

	txn1.Set([]byte("key1"), []byte("value1"))
	assert.Equal(t, db.ErrKeyNotFound, txn2.Get([]byte("key1"), noop))

	assert.NoError(t, txn1.Commit())
	assert.Equal(t, db.ErrKeyNotFound, txn2.Get([]byte("key1"), noop))
	txn2.Discard()

	txn3 := testDb.NewTransaction(false)
	assert.NoError(t, txn3.Get([]byte("key1"), func(bytes []byte) error {
		assert.Equal(t, []byte("value1"), bytes)
		return nil
	}))
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
			assert.Equal(t, 0, len(val))
			return nil
		})
		return err
	})
	assert.Nil(t, err)
}

func TestConcurrentUpdate(t *testing.T) {
	testDb := db.NewTestDb()
	defer testDb.Close()
	wg := sync.WaitGroup{}

	key := []byte{0}
	require.NoError(t, testDb.Update(func(txn db.Transaction) error {
		return txn.Set(key, []byte{0})
	}))
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				assert.NoError(t, testDb.Update(func(txn db.Transaction) error {
					var next byte
					err := txn.Get(key, func(bytes []byte) error {
						next = bytes[0] + 1
						return nil
					})
					if err != nil {
						return err
					}
					return txn.Set(key, []byte{next})
				}))
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				txn := testDb.NewTransaction(true)
				var next byte
				require.NoError(t, txn.Get(key, func(bytes []byte) error {
					next = bytes[0] + 1
					return nil
				}))
				require.NoError(t, txn.Set(key, []byte{next}))
				txn.Commit()
			}
		}()
	}

	wg.Wait()
	require.NoError(t, testDb.View(func(txn db.Transaction) error {
		return txn.Get(key, func(bytes []byte) error {
			assert.Equal(t, byte(200), bytes[0])
			return nil
		})
	}))
}
