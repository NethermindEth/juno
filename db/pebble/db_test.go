package pebble_test

import (
	"encoding/binary"
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

func TestSeek(t *testing.T) {
	testDb := pebble.NewMemTest()
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
		t.Run("seeks to the next key in lexicographical order", func(t *testing.T) {
			iter := txn.NewIterator(db.IterOptions{
				LowerBound: []byte{0},
				UpperBound: []byte{2},
			})
			defer iter.Close()

			err := iter.Seek(func(next *db.Entry) error {
				assert.Equal(t, []byte{1}, next.Key)
				assert.Equal(t, []byte{1}, next.Value)
				return nil
			})
			assert.NoError(t, err)
		})

		t.Run("stop iteration when next key is upper bound", func(t *testing.T) {
			iter := txn.NewIterator(db.IterOptions{
				LowerBound: []byte{1},
				UpperBound: []byte{3},
			})
			defer iter.Close()

			key := 0
			err = iter.Seek(func(next *db.Entry) error {
				key++
				return nil
			})
			assert.NoError(t, err)
			assert.Equal(t, 1, key)
		})

		t.Run("return nil when key is not found", func(t *testing.T) {
			iter := txn.NewIterator(db.IterOptions{
				LowerBound: []byte{4},
				UpperBound: []byte{5},
			})
			defer iter.Close()

			err = iter.Seek(func(next *db.Entry) error {
				assert.Nil(t, next)
				return nil
			})
			assert.NoError(t, err)
		})
		return nil
	})
}

type Entry struct {
	Key   uint64
	Value []byte
}

func TestIterator(t *testing.T) {
	data := []struct {
		key   uint64
		value []byte
	}{
		{11, []byte("c")},
		{12, []byte("a")},
		{13, []byte("e")},
		{22, []byte("d")},
		{23, []byte("b")},
		{123, []byte("f")},
		{123678, []byte("f")},
	}

	testDb := pebble.NewMemTest()
	defer testDb.Close()

	require.NoError(t, testDb.Update(func(txn db.Transaction) error {
		for _, d := range data {
			numBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(numBytes, d.key)
			err := txn.Set(numBytes, d.value)
			assert.NoError(t, err)
		}
		return nil
	}))

	require.NoError(t, testDb.View(func(txn db.Transaction) error {
		lowBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(lowBytes, 10)
		highBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(highBytes, 20)
		iter := txn.NewIterator(db.IterOptions{
			LowerBound: lowBytes,
			UpperBound: highBytes,
		})
		defer iter.Close()

		prefixBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(prefixBytes, 1)
		var entries []Entry
		err := iter.Seek(func(e *db.Entry) error {
			var key uint64
			keyBytes := make([]byte, 8)
			copy(keyBytes[:], iter.Key())
			key = binary.BigEndian.Uint64(keyBytes)
			entries = append(entries, Entry{
				Key:   key,
				Value: iter.Value(),
			})

			return nil
		})
		assert.NoError(t, err)

		expectedKeys := []uint64{11, 12, 13}

		assert.Equal(t, len(expectedKeys), len(entries))

		for i := 0; i < len(entries); i++ {
			assert.Contains(t, expectedKeys, entries[i].Key)
		}

		return nil
	}))
}
