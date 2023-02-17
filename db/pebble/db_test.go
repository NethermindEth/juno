package pebble_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var noop = func(val []byte) error {
	return nil
}

func TestTransaction(t *testing.T) {
	t.Run("new transaction can retrieve exising value", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		txn := testDb.NewTransaction(true)
		require.NoError(t, txn.Set([]byte("key"), []byte("value")))

		require.NoError(t, txn.Commit())

		readOnlyTxn := testDb.NewTransaction(false)
		assert.NoError(t, readOnlyTxn.Get([]byte("key"), func(val []byte) error {
			assert.Equal(t, "value", string(val))
			return nil
		}))
	})

	t.Run("discarded transaction is not committed to DB", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		txn := testDb.NewTransaction(true)
		require.NoError(t, txn.Set([]byte("key"), []byte("value")))
		txn.Discard()

		readOnlyTxn := testDb.NewTransaction(false)
		assert.EqualError(t, readOnlyTxn.Get([]byte("key"), noop), db.ErrKeyNotFound.Error())
	})

	t.Run("value committed by a transactions are not accessible to other transactions created"+
		" before Commit()", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		txn1 := testDb.NewTransaction(true)
		txn2 := testDb.NewTransaction(false)

		require.NoError(t, txn1.Set([]byte("key1"), []byte("value1")))
		assert.EqualError(t, txn2.Get([]byte("key1"), noop), db.ErrKeyNotFound.Error())

		require.NoError(t, txn1.Commit())
		assert.EqualError(t, txn2.Get([]byte("key1"), noop), db.ErrKeyNotFound.Error())
		txn2.Discard()

		txn3 := testDb.NewTransaction(false)
		assert.NoError(t, txn3.Get([]byte("key1"), func(bytes []byte) error {
			assert.Equal(t, []byte("value1"), bytes)
			return nil
		}))
	})

	t.Run("discarded transaction cannot commit", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		txn := testDb.NewTransaction(true)
		require.NoError(t, txn.Set([]byte("key"), []byte("value")))
		txn.Discard()

		assert.Error(t, txn.Commit())
	})
}

func TestViewUpdate(t *testing.T) {
	t.Run("value after Update is committed to DB", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		// Test View
		require.EqualError(t, testDb.View(func(txn db.Transaction) error {
			return txn.Get([]byte("key"), noop)
		}), db.ErrKeyNotFound.Error())

		// Test Update
		require.NoError(t, testDb.Update(func(txn db.Transaction) error {
			return txn.Set([]byte("key"), []byte("value"))
		}))

		// Check value
		assert.NoError(t, testDb.View(func(txn db.Transaction) error {
			err := txn.Get([]byte("key"), func(val []byte) error {
				assert.Equal(t, "value", string(val))
				return nil
			})
			return err
		}))
	})

	t.Run("Update error does not commit value to DB", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		// Test Update
		require.EqualError(t, testDb.Update(func(txn db.Transaction) error {
			err := txn.Set([]byte("key"), []byte("value"))
			assert.Nil(t, err)
			return fmt.Errorf("error")
		}), fmt.Errorf("error").Error())

		// Check key is not in the db
		assert.EqualError(t, testDb.View(func(txn db.Transaction) error {
			return txn.Get([]byte("key"), noop)
		}), db.ErrKeyNotFound.Error())
	})

	t.Run("setting a key with a zero-length value should be allowed", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		assert.NoError(t, testDb.Update(func(txn db.Transaction) error {
			require.NoError(t, txn.Set([]byte("key"), []byte{}))

			return txn.Get([]byte("key"), func(val []byte) error {
				assert.Equal(t, []byte{}, val)
				return nil
			})
		}))
	})

	t.Run("setting a key with a nil value should be allowed", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		assert.NoError(t, testDb.Update(func(txn db.Transaction) error {
			require.NoError(t, txn.Set([]byte("key"), nil))

			return txn.Get([]byte("key"), func(val []byte) error {
				assert.Equal(t, 0, len(val))
				return nil
			})
		}))
	})

	t.Run("setting a key with a zero-length key should not be allowed", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		assert.Error(t, testDb.Update(func(txn db.Transaction) error {
			return txn.Set([]byte{}, []byte("value"))
		}))
	})

	t.Run("setting a key with a nil key should not be allowed", func(t *testing.T) {
		testDb := pebble.NewMemTest()
		defer testDb.Close()

		assert.Error(t, testDb.Update(func(txn db.Transaction) error {
			return txn.Set(nil, []byte("value"))
		}))
	})
}

func TestConcurrentUpdate(t *testing.T) {
	testDb := pebble.NewMemTest()
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
				require.NoError(t, txn.Commit())
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
