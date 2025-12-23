package db

import (
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKeyValueStoreSuite runs a suite of tests against a KeyValueStore database
// implementation.
//
//nolint:mnd,gocyclo,funlen
func TestKeyValueStoreSuite(t *testing.T, newDB func() KeyValueStore) {
	t.Run("Iterator", func(t *testing.T) {
		tests := []struct {
			name    string
			content map[string]string
			prefix  string
			start   string
			order   []string
		}{
			{
				name:    "Empty database",
				content: map[string]string{},
				prefix:  "",
				start:   "",
				order:   nil,
			},
			{
				name:    "Empty database with non-existent prefix",
				content: map[string]string{},
				prefix:  "non-existent-prefix",
				start:   "",
				order:   nil,
			},
			{
				name:    "Single-item database",
				content: map[string]string{"key": "val"},
				prefix:  "",
				start:   "",
				order:   []string{"key"},
			},
			{
				name:    "Single-item database with matching prefix",
				content: map[string]string{"key": "val"},
				prefix:  "k",
				start:   "",
				order:   []string{"key"},
			},
			{
				name:    "Single-item database with non-matching prefix",
				content: map[string]string{"key": "val"},
				prefix:  "l",
				start:   "",
				order:   nil,
			},
			{
				name:    "Multi-item database",
				content: map[string]string{"k1": "v1", "k5": "v5", "k2": "v2", "k4": "v4", "k3": "v3"},
				prefix:  "",
				start:   "",
				order:   []string{"k1", "k2", "k3", "k4", "k5"},
			},
			{
				name:    "Multi-item database with matching prefix",
				content: map[string]string{"k1": "v1", "k5": "v5", "k2": "v2", "k4": "v4", "k3": "v3"},
				prefix:  "k",
				start:   "",
				order:   []string{"k1", "k2", "k3", "k4", "k5"},
			},
			{
				name:    "Multi-item database with non-matching prefix",
				content: map[string]string{"k1": "v1", "k5": "v5", "k2": "v2", "k4": "v4", "k3": "v3"},
				prefix:  "l",
				start:   "",
				order:   nil,
			},
			{
				name: "Multi-item database with specific prefix",
				content: map[string]string{
					"ka1": "va1", "ka5": "va5", "ka2": "va2", "ka4": "va4", "ka3": "va3",
					"kb1": "vb1", "kb5": "vb5", "kb2": "vb2", "kb4": "vb4", "kb3": "vb3",
				},
				prefix: "ka",
				start:  "",
				order:  []string{"ka1", "ka2", "ka3", "ka4", "ka5"},
			},
			{
				name: "Multi-item database with non-existent prefix",
				content: map[string]string{
					"ka1": "va1", "ka5": "va5", "ka2": "va2", "ka4": "va4", "ka3": "va3",
					"kb1": "vb1", "kb5": "vb5", "kb2": "vb2", "kb4": "vb4", "kb3": "vb3",
				},
				prefix: "kc",
				start:  "",
				order:  nil,
			},
			{
				name: "Multi-item database with prefix and start position",
				content: map[string]string{
					"ka1": "va1", "ka5": "va5", "ka2": "va2", "ka4": "va4", "ka3": "va3",
					"kb1": "vb1", "kb5": "vb5", "kb2": "vb2", "kb4": "vb4", "kb3": "vb3",
				},
				prefix: "ka",
				start:  "ka3",
				order:  []string{"ka3", "ka4", "ka5"},
			},
			{
				name: "Multi-item database with prefix and out-of-range start position",
				content: map[string]string{
					"ka1": "va1", "ka5": "va5", "ka2": "va2", "ka4": "va4", "ka3": "va3",
					"kb1": "vb1", "kb5": "vb5", "kb2": "vb2", "kb4": "vb4", "kb3": "vb3",
				},
				prefix: "ka",
				start:  "ka8",
				order:  nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Create the key-value data store
				database := newDB()
				defer database.Close()

				for key, val := range tt.content {
					err := database.Put([]byte(key), []byte(val))
					require.NoError(t, err, "failed to insert item %s:%s into database", key, val)
				}

				// Iterate over the database with the given configs and verify the results
				var startBytes []byte
				if tt.start != "" {
					startBytes = []byte(tt.start)
				}

				it, err := database.NewIterator([]byte(tt.prefix), true)
				require.NoError(t, err, "failed to create iterator")
				defer it.Close()

				if startBytes != nil {
					it.Seek(startBytes)
				} else {
					it.First()
				}

				var keys []string
				for ; it.Valid(); it.Next() {
					keys = append(keys, string(it.Key()))
					val, err := it.Value()
					require.NoError(t, err, "failed to get value")
					require.Equal(t, tt.content[string(it.Key())], string(val), "value mismatch")

					uncopiedVal, err := it.UncopiedValue()
					require.NoError(t, err, "failed to get value")
					require.Equal(
						t,
						tt.content[string(it.Key())],
						string(uncopiedVal),
						"value mismatch",
					)
					require.NotSame(
						t,
						&val[0],
						&uncopiedVal[0],
						"Iterator value should be copied to a new slice",
					)

					uncopiedVal2, err := it.UncopiedValue()
					require.NoError(t, err, "failed to get value")
					require.Equal(
						t,
						tt.content[string(it.Key())],
						string(uncopiedVal2),
						"value mismatch",
					)
					require.Same(
						t,
						&uncopiedVal[0],
						&uncopiedVal2[0],
						"Iterator value should be copied to a new slice",
					)
				}

				// Sort the expected order for comparison
				expectedOrder := make([]string, len(tt.order))
				copy(expectedOrder, tt.order)
				sort.Strings(expectedOrder)

				// Sort the actual keys for comparison
				sort.Strings(keys)

				for i, key := range keys {
					require.Equal(t, expectedOrder[i], key, "expected order mismatch")
				}
			})
		}
	})

	t.Run("KeyValueOperations", func(t *testing.T) {
		database := newDB()
		defer database.Close()

		key := []byte("foo")

		// Test Has on non-existent key
		has, err := database.Has(key)
		require.NoError(t, err, "Has operation failed")
		require.False(t, has, "key should not exist")

		// Test Get on non-existent key
		err = database.Get(key, func([]byte) error { return nil })
		require.Error(t, err, "Get should fail on non-existent key %s", key)
		require.Equal(t, ErrKeyNotFound, err, "expected ErrKeyNotFound")

		// Test Put
		value := []byte("hello world")
		err = database.Put(key, value)
		require.NoError(t, err, "Put operation failed")

		// Test Has after Put
		has, err = database.Has(key)
		require.NoError(t, err, "Has operation failed for key %s", key)
		require.True(t, has, "key should exist after Put")

		// Test Get after Put
		var got []byte
		err = database.Get(key, func(data []byte) error {
			got = data
			return nil
		})
		require.NoError(t, err, "Get operation failed for key %s", key)
		require.Equal(t, value, got, "Get returned wrong value for key %s, expected %s, got %s", key, value, got)

		// Test Delete
		err = database.Delete(key)
		require.NoError(t, err, "Delete operation failed for key %s", key)

		// Test Has after Delete
		has, err = database.Has(key)
		require.NoError(t, err, "Has operation failed for key %s", key)
		require.False(t, has, "key should not exist after Delete for key %s", key)

		// Test Get after Delete
		err = database.Get(key, func([]byte) error { return nil })
		require.Error(t, err, "Get should fail after Delete for key %s", key)
		require.Equal(t, ErrKeyNotFound, err, "expected ErrKeyNotFound")
	})

	t.Run("Batch", func(t *testing.T) {
		database := newDB()
		defer database.Close()

		// Create a batch
		batch := database.NewBatch()
		require.NotNil(t, batch, "NewBatch should return a non-nil batch")

		// Add some data to the batch
		testData := map[string]string{
			"batch-key1": "batch-value1",
			"batch-key2": "batch-value2",
			"batch-key3": "batch-value3",
		}

		for k, v := range testData {
			err := batch.Put([]byte(k), []byte(v))
			require.NoError(t, err, "Put operation on batch failed for key %s", k)
		}

		// Check batch size is non-zero
		require.Greater(t, batch.Size(), 0, "Batch size should be greater than 0")

		// Verify data is not in database before write
		for k := range testData {
			has, err := database.Has([]byte(k))
			require.NoError(t, err, "Has operation failed for key %s", k)
			require.False(t, has, "key should not exist in database before batch write for key %s", k)
		}

		// Write batch to database
		err := batch.Write()
		require.NoError(t, err, "Batch write failed")

		// Verify data is in database after write
		for k, v := range testData {
			var val []byte
			err = database.Get([]byte(k), func(data []byte) error {
				val = data
				return nil
			})
			require.NoError(t, err, "Get operation failed for key %s", k)
			require.Equal(t, []byte(v), val, "Value mismatch after batch write for key %s, expected %s, got %s", k, v, val)
		}

		// Test batch reset
		batch.Reset()
		require.Equal(t, 0, batch.Size(), "Batch size should be 0 after reset")

		// Test batch with pre-allocated size
		batchWithSize := database.NewBatchWithSize(1024)
		require.NotNil(t, batchWithSize, "NewBatchWithSize should return a non-nil batch")

		// Add data to the batch with size
		key := "batch-size-key"
		value := "batch-size-value"
		err = batchWithSize.Put([]byte(key), []byte(value))
		require.NoError(t, err, "Put operation on batch with size failed for key %s", key)

		// Write batch with size to database
		err = batchWithSize.Write()
		require.NoError(t, err, "Batch with size write failed")

		// Verify data is in database
		var got []byte
		err = database.Get([]byte(key), func(data []byte) error {
			got = data
			return nil
		})
		require.NoError(t, err, "Get operation failed for key %s", key)
		require.Equal(t, []byte(value), got, "Value mismatch after batch with size write for key %s, expected %s, got %s", key, value, got)

		// Test batch delete operation
		deleteBatch := database.NewBatch()
		key = "batch-key1"
		err = deleteBatch.Delete([]byte(key))
		require.NoError(t, err, "Delete operation on batch failed for key %s", key)
		err = deleteBatch.Write()
		require.NoError(t, err, "Batch write failed")

		// Verify key was deleted
		has, err := database.Has([]byte(key))
		require.NoError(t, err, "Has operation failed for key %s", key)
		require.False(t, has, "Key should not exist after batch delete for key %s", key)
	})

	t.Run("IndexedBatch", func(t *testing.T) {
		database := newDB()
		defer database.Close()

		// Create an indexed batch
		batch := database.NewIndexedBatch()
		require.NotNil(t, batch, "NewIndexedBatch should return a non-nil batch")

		// Add some data to the batch
		testData := map[string]string{
			"indexed-key1": "indexed-value1",
			"indexed-key2": "indexed-value2",
			"indexed-key3": "indexed-value3",
		}

		for k, v := range testData {
			err := batch.Put([]byte(k), []byte(v))
			require.NoError(t, err, "Put operation on indexed batch failed for key %s", k)
		}

		// Check batch size is non-zero
		require.Greater(t, batch.Size(), 0, "Indexed batch size should be greater than 0")

		// Test read operations on the batch (should see uncommitted data)
		for k, v := range testData {
			// Test Has
			has, err := batch.Has([]byte(k))
			require.NoError(t, err, "Has operation on indexed batch failed for key %s", k)
			require.True(t, has, "key %s should exist in indexed batch", k)

			// Test Get
			var val []byte
			err = batch.Get([]byte(k), func(data []byte) error {
				val = data
				return nil
			})
			require.NoError(t, err, "Get operation on indexed batch failed for key %s", k)
			require.Equal(t, []byte(v), val, "value mismatch for key %s in indexed batch", k)
		}

		// Test iterator on the batch
		iter, err := batch.NewIterator([]byte("indexed-"), false)
		require.NoError(t, err, "NewIterator on indexed batch failed")
		defer iter.Close()

		// Count keys with prefix
		count := 0
		for iter.First(); iter.Valid(); iter.Next() {
			count++
			key := string(iter.Key())
			require.Contains(t, testData, key, "Iterator returned unexpected key %s", key)
			val, err := iter.Value()
			require.NoError(t, err, "Iterator value retrieval failed for key %s", key)
			require.Equal(t, []byte(testData[key]), val, "Iterator value mismatch for key %s", key)

			uncopiedVal, err := iter.UncopiedValue()
			require.NoError(t, err, "Iterator uncopied value retrieval failed for key %s", key)
			require.Equal(
				t,
				[]byte(testData[key]),
				uncopiedVal,
				"Iterator uncopied value mismatch for key %s",
				key,
			)
			require.NotSame(
				t,
				&val[0],
				&uncopiedVal[0],
				"Iterator value should be copied to a new slice",
			)

			uncopiedVal2, err := iter.UncopiedValue()
			require.NoError(t, err, "Iterator uncopied value retrieval failed for key %s", key)
			require.Equal(
				t,
				[]byte(testData[key]),
				uncopiedVal2,
				"Iterator uncopied value mismatch for key %s",
				key,
			)
			require.Same(
				t,
				&uncopiedVal[0],
				&uncopiedVal2[0],
				"Iterator uncopied values should be the same",
			)
		}
		require.Equal(t, len(testData), count, "Iterator should return all keys with prefix")

		// Write batch to database
		err = batch.Write()
		require.NoError(t, err, "Indexed batch write failed")

		// Verify data is in database after write
		for k, v := range testData {
			var val []byte
			err = database.Get([]byte(k), func(data []byte) error {
				val = data
				return nil
			})
			require.NoError(t, err, "Get operation failed for key %s", k)
			require.Equal(t, []byte(v), val, "Value mismatch after indexed batch write for key %s", k)
		}

		// Test batch with pre-allocated size
		batchWithSize := database.NewIndexedBatchWithSize(1024)
		require.NotNil(t, batchWithSize, "NewIndexedBatchWithSize should return a non-nil batch")

		// Add and read data in the same batch
		key := "indexed-size-key"
		value := "indexed-size-value"
		err = batchWithSize.Put([]byte(key), []byte(value))
		require.NoError(t, err, "Put operation on indexed batch with size failed for key %s", key)

		// Verify data can be read from the batch before writing
		var val []byte
		err = batchWithSize.Get([]byte(key), func(data []byte) error {
			val = data
			return nil
		})
		require.NoError(t, err, "Get operation on indexed batch with size failed for key %s", key)
		require.Equal(t, []byte(value), val, "Value mismatch in indexed batch with size for key %s", key)

		// Write batch with size to database
		err = batchWithSize.Write()
		require.NoError(t, err, "Indexed batch with size write failed")

		// Test batch delete and read in the same batch
		deleteBatch := database.NewIndexedBatch()

		// First add a key
		key = "to-delete"
		value = "delete-me"
		err = deleteBatch.Put([]byte(key), []byte(value))
		require.NoError(t, err, "Put operation on indexed batch failed for key %s", key)

		// Then delete it in the same batch
		err = deleteBatch.Delete([]byte(key))
		require.NoError(t, err, "Delete operation on indexed batch failed for key %s", key)

		// Verify the key is not visible in the batch
		has, err := deleteBatch.Has([]byte(key))
		require.NoError(t, err, "Has operation on indexed batch failed for key %s", key)
		require.False(t, has, "key %s should not exist in indexed batch after delete", key)

		// Write the batch
		err = deleteBatch.Write()
		require.NoError(t, err, "Indexed batch write failed")
	})

	t.Run("BatchSize", func(t *testing.T) {
		database := newDB()
		defer database.Close()

		batch := database.NewBatch()

		// Empty batch should have size 0
		require.Equal(t, 0, batch.Size(), "empty batch should have size 0")

		// Add some entries and check size increases
		err := batch.Put([]byte("key1"), []byte("value1"))
		require.NoError(t, err, "batch Put failed")
		require.Greater(t, batch.Size(), 0, "batch size should be greater than 0 after Put")

		initialSize := batch.Size()

		err = batch.Put([]byte("key2"), []byte("value2"))
		require.NoError(t, err, "batch Put failed")
		require.Greater(t, batch.Size(), initialSize, "batch size should increase after second Put")
	})

	t.Run("DeleteRange", func(t *testing.T) {
		database := newDB()
		defer database.Close()

		// Helper to add a range of keys
		addRange := func(start, stop int) {
			for i := start; i <= stop; i++ {
				key := strconv.Itoa(i)
				value := "value-" + key
				err := database.Put([]byte(key), []byte(value))
				require.NoError(t, err, "Put operation failed for key %s", key)
			}
		}

		// Helper to check if a range of keys exists
		checkRange := func(start, stop int, expected bool) {
			for i := start; i <= stop; i++ {
				key := []byte(strconv.Itoa(i))
				has, err := database.Has(key)
				require.NoError(t, err, "Has operation failed for key %s", key)
				if expected {
					require.True(t, has, "key %s should exist", key)
				} else {
					require.False(t, has, "key %s should not exist", key)
				}
			}
		}

		addRange(1, 9)
		require.NoError(t, database.DeleteRange([]byte("9"), []byte("1"))) // no-op
		checkRange(1, 9, true)
		require.NoError(t, database.DeleteRange([]byte("5"), []byte("5"))) // no-op, exclusive end
		checkRange(1, 9, true)
		require.NoError(t, database.DeleteRange([]byte("5"), []byte("50"))) // delete only key 5
		checkRange(1, 4, true)
		checkRange(5, 5, false)
		checkRange(6, 9, true)
		require.NoError(t, database.DeleteRange([]byte(""), []byte("a"))) // delete all
		checkRange(1, 9, false)

		addRange(1, 999)
		require.NoError(t, database.DeleteRange([]byte("12345"), []byte("54321")))
		checkRange(1, 1, true)
		checkRange(2, 5, false)
		checkRange(6, 12, true)
		checkRange(13, 54, false)
		checkRange(55, 123, true)
		checkRange(124, 543, false)
		checkRange(544, 999, true)

		addRange(1, 999)
		require.NoError(t, database.DeleteRange([]byte("3"), []byte("7")))
		checkRange(1, 2, true)
		checkRange(3, 6, false)
		checkRange(7, 29, true)
		checkRange(30, 69, false)
		checkRange(70, 299, true)
		checkRange(300, 699, false)
		checkRange(700, 999, true)

		require.NoError(t, database.DeleteRange([]byte(""), []byte("a")))
		checkRange(1, 999, false)
	})

	t.Run("Snapshot", func(t *testing.T) {
		database := newDB()
		defer database.Close()

		// Add some initial data
		initialData := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}

		for k, v := range initialData {
			err := database.Put([]byte(k), []byte(v))
			require.NoError(t, err, "Put operation failed for key %s", k)
		}

		// Create a snapshot
		snapshot := database.NewSnapshot()
		// Note: Snapshot doesn't have a Close method in the interface

		// Modify the database after snapshot
		key := "key4"
		value := "value4"
		err := database.Put([]byte(key), []byte(value))
		require.NoError(t, err, "Put operation failed for key %s", key)

		key = "key1"
		err = database.Delete([]byte(key))
		require.NoError(t, err, "Delete operation failed for key %s", key)

		key = "key2"
		value = "modified-value2"
		err = database.Put([]byte(key), []byte(value))
		require.NoError(t, err, "Put operation failed for key %s", key)

		// Check snapshot has original data
		for k, v := range initialData {
			var val []byte
			err = snapshot.Get([]byte(k), func(data []byte) error {
				val = data
				return nil
			})
			require.NoError(t, err, "Get from snapshot failed for key %s", k)
			require.Equal(t, []byte(v), val, "snapshot value mismatch for key %s", k)
		}

		// Snapshot should not have new key
		err = snapshot.Get([]byte("key4"), func([]byte) error { return nil })
		require.Error(t, err, "key4 should not exist in snapshot")
		require.Equal(t, ErrKeyNotFound, err, "expected ErrKeyNotFound")

		// Check database has modified data
		// key1 should be deleted
		has, err := database.Has([]byte("key1"))
		require.NoError(t, err, "Has operation failed for key1")
		require.False(t, has, "key1 should be deleted in database")

		// key2 should have modified value
		var val []byte
		err = database.Get([]byte("key2"), func(data []byte) error {
			val = data
			return nil
		})
		require.NoError(t, err, "Get operation failed for key2")
		require.Equal(t, []byte("modified-value2"), val, "key2 should have modified value")

		// key4 should exist
		err = database.Get([]byte("key4"), func(data []byte) error {
			val = data
			return nil
		})
		require.NoError(t, err, "Get operation failed for key4")
		require.Equal(t, []byte("value4"), val, "key4 should exist in database")

		// Test snapshot iterator
		it, err := snapshot.NewIterator(nil, false)
		require.NoError(t, err, "failed to create iterator")
		defer it.Close()

		var keys []string
		for it.First(); it.Valid(); it.Next() {
			keys = append(keys, string(it.Key()))
		}
		sort.Strings(keys)

		expectedKeys := []string{"key1", "key2", "key3"}
		require.Equal(t, expectedKeys, keys, "snapshot iterator keys mismatch")
	})

	t.Run("OperationsAfterClose", func(t *testing.T) {
		database := newDB()

		// Add a key before closing
		key := "key"
		value := "value"
		err := database.Put([]byte(key), []byte(value))
		require.NoError(t, err, "Put operation failed for key %s", key)

		// Close the database
		err = database.Close()
		require.NoError(t, err, "Close operation failed")

		// Operations should fail after close
		err = database.Get([]byte(key), func([]byte) error { return nil })
		require.Error(t, err, "Get should fail after Close for key %s", key)

		_, err = database.Has([]byte(key))
		require.Error(t, err, "Has should fail after Close for key %s", key)

		err = database.Put([]byte("key2"), []byte("value2"))
		require.Error(t, err, "Put should fail after Close for key %s", key)

		err = database.Delete([]byte(key))
		require.Error(t, err, "Delete should fail after Close for key %s", key)

		err = database.DeleteRange([]byte("key"), []byte("key2"))
		require.Error(t, err, "DeleteRange should fail after Close")

		_, err = database.NewIterator(nil, false)
		require.Error(t, err, "NewIterator should fail after Close")

		// Batch operations
		batch := database.NewBatch()
		// Put might not fail immediately since batch operations are buffered
		_ = batch.Put([]byte("batchkey"), []byte("batchval"))

		// But Write should fail
		err = batch.Write()
		require.Error(t, err, "batch Write should fail after Close")

		// Same with indexed batch
		batch = database.NewIndexedBatch()
		_ = batch.Put([]byte("batchkey"), []byte("batchval"))
		err = batch.Write()
		require.Error(t, err, "batch Write should fail after Close")

		// NewSnapshot() on closed db should panic
		require.Panics(t, func() {
			database.NewSnapshot()
		})
	})
}
