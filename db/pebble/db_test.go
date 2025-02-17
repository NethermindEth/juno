package pebble_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var noop = func(val []byte) error {
	return nil
}

type eventListener struct {
	WriteCount int
	ReadCount  int
}

func (l *eventListener) OnIO(write bool, _ time.Duration) {
	if write {
		l.WriteCount++
	} else {
		l.ReadCount++
	}
}

func (l *eventListener) OnCommit(_ time.Duration) {}

func TestTransaction(t *testing.T) {
	listener := eventListener{}
	t.Run("new transaction can retrieve existing value", func(t *testing.T) {
		testDB := pebble.NewMemTest(t).WithListener(&listener)

		txn, err := testDB.NewTransaction(true)
		require.NoError(t, err)
		require.NoError(t, txn.Set([]byte("key"), []byte("value")))
		assert.Equal(t, 1, listener.WriteCount)
		assert.Equal(t, 0, listener.ReadCount)

		require.NoError(t, txn.Commit())

		readOnlyTxn, err := testDB.NewTransaction(false)
		require.NoError(t, err)
		assert.NoError(t, readOnlyTxn.Get([]byte("key"), func(val []byte) error {
			assert.Equal(t, "value", string(val))
			return nil
		}))
		assert.Equal(t, 1, listener.WriteCount)
		assert.Equal(t, 1, listener.ReadCount)

		require.NoError(t, readOnlyTxn.Discard())
	})

	t.Run("discarded transaction is not committed to DB", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)

		txn, err := testDB.NewTransaction(true)
		require.NoError(t, err)
		require.NoError(t, txn.Set([]byte("key"), []byte("value")))
		require.NoError(t, txn.Discard())

		readOnlyTxn, err := testDB.NewTransaction(false)
		require.NoError(t, err)
		assert.EqualError(t, readOnlyTxn.Get([]byte("key"), noop), db.ErrKeyNotFound.Error())
		require.NoError(t, readOnlyTxn.Discard())
	})

	t.Run("value committed by a transactions are not accessible to other transactions created"+
		" before Commit()", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)

		txn1, err := testDB.NewTransaction(true)
		require.NoError(t, err)
		txn2, err := testDB.NewTransaction(false)
		require.NoError(t, err)

		require.NoError(t, txn1.Set([]byte("key1"), []byte("value1")))
		assert.EqualError(t, txn2.Get([]byte("key1"), noop), db.ErrKeyNotFound.Error())

		require.NoError(t, txn1.Commit())
		assert.EqualError(t, txn2.Get([]byte("key1"), noop), db.ErrKeyNotFound.Error())
		require.NoError(t, txn2.Discard())

		txn3, err := testDB.NewTransaction(false)
		require.NoError(t, err)
		assert.NoError(t, txn3.Get([]byte("key1"), func(bytes []byte) error {
			assert.Equal(t, []byte("value1"), bytes)
			return nil
		}))
		require.NoError(t, txn3.Discard())
	})

	t.Run("discarded transaction cannot commit", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)

		txn, err := testDB.NewTransaction(true)
		require.NoError(t, err)
		require.NoError(t, txn.Set([]byte("key"), []byte("value")))
		require.NoError(t, txn.Discard())

		assert.Error(t, txn.Commit())
	})
}

func TestViewUpdate(t *testing.T) {
	t.Run("value after Update is committed to DB", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)

		// Test View
		require.EqualError(t, testDB.View(func(txn db.Transaction) error {
			return txn.Get([]byte("key"), noop)
		}), db.ErrKeyNotFound.Error())

		// Test Update
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			return txn.Set([]byte("key"), []byte("value"))
		}))

		// Check value
		assert.NoError(t, testDB.View(func(txn db.Transaction) error {
			err := txn.Get([]byte("key"), func(val []byte) error {
				assert.Equal(t, "value", string(val))
				return nil
			})
			return err
		}))
	})

	t.Run("Update error does not commit value to DB", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)

		// Test Update
		require.EqualError(t, testDB.Update(func(txn db.Transaction) error {
			err := txn.Set([]byte("key"), []byte("value"))
			assert.Nil(t, err)
			return fmt.Errorf("error")
		}), fmt.Errorf("error").Error())

		// Check key is not in the db
		assert.EqualError(t, testDB.View(func(txn db.Transaction) error {
			return txn.Get([]byte("key"), noop)
		}), db.ErrKeyNotFound.Error())
	})

	t.Run("setting a key with a zero-length value should be allowed", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)

		assert.NoError(t, testDB.Update(func(txn db.Transaction) error {
			require.NoError(t, txn.Set([]byte("key"), []byte{}))

			return txn.Get([]byte("key"), func(val []byte) error {
				assert.Equal(t, []byte{}, val)
				return nil
			})
		}))
	})

	t.Run("setting a key with a nil value should be allowed", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)

		assert.NoError(t, testDB.Update(func(txn db.Transaction) error {
			require.NoError(t, txn.Set([]byte("key"), nil))

			return txn.Get([]byte("key"), func(val []byte) error {
				assert.Equal(t, 0, len(val))
				return nil
			})
		}))
	})

	t.Run("setting a key with a zero-length key should not be allowed", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)

		assert.Error(t, testDB.Update(func(txn db.Transaction) error {
			return txn.Set([]byte{}, []byte("value"))
		}))
	})

	t.Run("setting a key with a nil key should not be allowed", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)

		assert.Error(t, testDB.Update(func(txn db.Transaction) error {
			return txn.Set(nil, []byte("value"))
		}))
	})
}

func TestConcurrentUpdate(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	wg := sync.WaitGroup{}

	key := []byte{0}
	require.NoError(t, testDB.Update(func(txn db.Transaction) error {
		return txn.Set(key, []byte{0})
	}))
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				assert.NoError(t, testDB.Update(func(txn db.Transaction) error {
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

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				txn, err := testDB.NewTransaction(true)
				require.NoError(t, err)
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
	require.NoError(t, testDB.View(func(txn db.Transaction) error {
		return txn.Get(key, func(bytes []byte) error {
			assert.Equal(t, byte(200), bytes[0])
			return nil
		})
	}))
}

func TestSeek(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	require.NoError(t, txn.Set([]byte{1}, []byte{1}))
	require.NoError(t, txn.Set([]byte{3}, []byte{3}))

	t.Run("seeks to the next key in lexicographical order", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, iter.Close())
		})

		iter.Seek([]byte{0})
		v, err := iter.Value()
		require.NoError(t, err)
		assert.Equal(t, []byte{1}, iter.Key())
		assert.Equal(t, []byte{1}, v)
	})

	t.Run("key returns nil when seeking nonexistent data", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, iter.Close())
		})

		iter.Seek([]byte{4})
		assert.Nil(t, iter.Key())
	})
}

func TestPrefixSearch(t *testing.T) {
	type entry struct {
		prefix []byte
		key    uint64
		value  []byte
	}

	data := []entry{
		{[]byte{11}, 1, []byte("c")},
		{[]byte{11}, 2, []byte("a")},
		{[]byte{11}, 3, []byte("e")},
		{[]byte{12}, 4, []byte("d")},
		{[]byte{23}, 5, []byte("b")},
		{[]byte{123}, 6, []byte("f")},
		{[]byte{0}, 7, []byte("g")},
	}

	testDB := pebble.NewMemTest(t)

	require.NoError(t, testDB.Update(func(txn db.Transaction) error {
		for _, d := range data {
			keyBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(keyBytes, d.key)
			var dbKey []byte
			dbKey = append(dbKey, d.prefix...)
			dbKey = append(dbKey, keyBytes...)
			require.NoError(t, txn.Set(dbKey, d.value))
		}
		return nil
	}))

	require.NoError(t, testDB.View(func(txn db.Transaction) error {
		targetPrefix := []byte{11}
		iter, err := txn.NewIterator(targetPrefix, true)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, iter.Close())
		})

		var entries []entry
		for iter.First(); iter.Valid(); iter.Next() {
			key := iter.Key()
			key = key[len(targetPrefix):]
			keyUint64 := binary.BigEndian.Uint64(key)

			v, err := iter.Value()
			require.NoError(t, err)

			entries = append(entries, entry{targetPrefix, keyUint64, v})
		}

		expectedKeys := []uint64{1, 2, 3}

		assert.Equal(t, len(expectedKeys), len(entries))

		for i := range entries {
			assert.Contains(t, expectedKeys, entries[i].key)
		}

		return nil
	}))
}

func TestFirst(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})
	require.NoError(t, txn.Set([]byte{0}, []byte{0}))
	require.NoError(t, txn.Set([]byte{1}, []byte{1}))
	require.NoError(t, txn.Set([]byte{2}, []byte{2}))

	t.Run("First() on new iterator", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		assert.Equal(t, true, iter.First())
		assert.Equal(t, []byte{0}, iter.Key())
		require.NoError(t, iter.Close())
	})

	t.Run("First() after multiple Next()", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		assert.Equal(t, true, iter.Next())
		assert.Equal(t, []byte{0}, iter.Key())
		assert.Equal(t, true, iter.Next())
		assert.Equal(t, []byte{1}, iter.Key())
		assert.Equal(t, true, iter.First())
		assert.Equal(t, []byte{0}, iter.Key())
		require.NoError(t, iter.Close())
	})
}

func TestPrev(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})

	t.Run("empty db", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		assert.Equal(t, false, iter.Prev())
		assert.Equal(t, []byte(nil), iter.Key())
		require.NoError(t, iter.Close())
	})

	one := []byte{1}
	two := []byte{2}
	three := []byte{3}
	require.NoError(t, txn.Set(one, one))
	require.NoError(t, txn.Set(two, two))
	require.NoError(t, txn.Set(three, three))

	t.Run("new iterator", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		assert.Equal(t, true, iter.Prev())
		assert.Equal(t, one, iter.Key())
		require.NoError(t, iter.Close())
	})

	t.Run("after valid seek", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		assert.Equal(t, true, iter.Seek(two))
		assert.Equal(t, two, iter.Key())
		assert.Equal(t, true, iter.Prev())
		assert.Equal(t, one, iter.Key())
		require.NoError(t, iter.Close())
	})

	t.Run("after invalid seek beyond last key", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		assert.Equal(t, false, iter.Seek([]byte{100}))
		assert.Equal(t, []byte(nil), iter.Key())
		assert.Equal(t, true, iter.Prev())
		assert.Equal(t, three, iter.Key())
		require.NoError(t, iter.Close())
	})

	t.Run("after valid seek first key", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		assert.Equal(t, true, iter.Seek(one))
		assert.Equal(t, one, iter.Key())
		assert.Equal(t, false, iter.Prev())
		assert.Equal(t, one, iter.Key())
		require.NoError(t, iter.Close())
	})

	t.Run("after multiple next", func(t *testing.T) {
		iter, err := txn.NewIterator(nil, false)
		require.NoError(t, err)
		assert.Equal(t, true, iter.Next())
		assert.Equal(t, one, iter.Key())
		assert.Equal(t, true, iter.Next())
		assert.Equal(t, two, iter.Key())
		assert.Equal(t, true, iter.Prev())
		assert.Equal(t, one, iter.Key())
		require.NoError(t, iter.Close())
	})

	t.Run("with lower bound", func(t *testing.T) {
		iter, err := txn.NewIterator(one, false)
		require.NoError(t, err)
		assert.Equal(t, true, iter.Seek(one))
		assert.Equal(t, one, iter.Key())
		assert.Equal(t, false, iter.Prev())
		assert.Equal(t, one, iter.Key())
		require.NoError(t, iter.Close())
	})
}

func TestNext(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	txn, err := testDB.NewTransaction(true)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, txn.Discard())
	})
	require.NoError(t, txn.Set([]byte{0}, []byte{0}))
	require.NoError(t, txn.Set([]byte{1}, []byte{1}))
	require.NoError(t, txn.Set([]byte{2}, []byte{2}))

	t.Run("Next() on new iterator", func(t *testing.T) {
		it, err := txn.NewIterator(nil, false)
		require.NoError(t, err)

		t.Run("new iterator should be invalid", func(t *testing.T) {
			assert.False(t, it.Valid())
		})

		t.Run("Next() should validate iterator", func(t *testing.T) {
			assert.True(t, it.Next())
		})

		require.NoError(t, it.Close())
	})

	t.Run("Next() should work as expected after a Seek()", func(t *testing.T) {
		it, err := txn.NewIterator(nil, false)
		require.NoError(t, err)

		require.True(t, it.Seek([]byte{0}))
		require.True(t, it.Next())
		require.Equal(t, []byte{1}, it.Key())

		require.NoError(t, it.Close())
	})
}

func TestPanic(t *testing.T) {
	testDB := pebble.NewMemTest(t)

	t.Run("view", func(t *testing.T) {
		defer func() {
			p := recover()
			require.NotNil(t, p)
		}()

		require.NoError(t, testDB.View(func(txn db.Transaction) error {
			panic("view")
		}))
	})

	t.Run("update", func(t *testing.T) {
		var panicingTxn db.Transaction
		defer func() {
			p := recover()
			require.NotNil(t, p)

			require.ErrorIs(t, testDB.View(func(txn db.Transaction) error {
				return txn.Get([]byte{0}, func(b []byte) error { return nil })
			}), db.ErrKeyNotFound)
			require.EqualError(t, panicingTxn.Get([]byte{0}, func(b []byte) error { return nil }), "discarded transaction")
		}()

		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			panicingTxn = txn
			require.ErrorIs(t, txn.Get([]byte{0}, func(b []byte) error { return nil }), db.ErrKeyNotFound)
			require.NoError(t, txn.Set([]byte{0}, []byte{0}))
			panic("update")
		}))
	})
}

func TestCalculatePrefixSize(t *testing.T) {
	t.Run("empty db", func(t *testing.T) {
		testDB := pebble.NewMemTest(t).(*pebble.DB)

		s, err := pebble.CalculatePrefixSize(context.Background(), testDB, []byte("0"), true)
		require.NoError(t, err)
		assert.Zero(t, s.Count)
		assert.Zero(t, s.Size)
	})

	t.Run("non empty db but empty prefix", func(t *testing.T) {
		testDB := pebble.NewMemTest(t)
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			return txn.Set(append([]byte("0"), []byte("randomKey")...), []byte("someValue"))
		}))
		s, err := pebble.CalculatePrefixSize(context.Background(), testDB.(*pebble.DB), []byte("1"), true)
		require.NoError(t, err)
		assert.Zero(t, s.Count)
		assert.Zero(t, s.Size)
	})

	t.Run("size of all key value pair with the same prefix", func(t *testing.T) {
		p := []byte("0")
		k1, v1 := append(p, []byte("key1")...), []byte("value1") //nolint: gocritic
		k2, v2 := append(p, []byte("key2")...), []byte("value2") //nolint: gocritic
		k3, v3 := append(p, []byte("key3")...), []byte("value3") //nolint: gocritic
		expectedSize := uint(len(k1) + len(v1) + len(k2) + len(v2) + len(k3) + len(v3))

		testDB := pebble.NewMemTest(t)
		require.NoError(t, testDB.Update(func(txn db.Transaction) error {
			require.NoError(t, txn.Set(k1, v1))
			require.NoError(t, txn.Set(k2, v2))
			return txn.Set(k3, v3)
		}))

		s, err := pebble.CalculatePrefixSize(context.Background(), testDB.(*pebble.DB), p, true)
		require.NoError(t, err)
		assert.Equal(t, uint(3), s.Count)
		assert.Equal(t, utils.DataSize(expectedSize), s.Size)

		t.Run("exit when context is cancelled", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			s, err := pebble.CalculatePrefixSize(ctx, testDB.(*pebble.DB), p, true)
			assert.EqualError(t, err, context.Canceled.Error())
			assert.Zero(t, s.Count)
			assert.Zero(t, s.Size)
		})
	})
}
