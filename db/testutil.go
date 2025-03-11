package db

import (
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKeyValueStoreSuite runs a suite of tests against a KeyValueStore database
// implementation.
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
		_, err = database.Get(key)
		require.Error(t, err, "Get should fail on non-existent key")
		require.Equal(t, ErrKeyNotFound, err, "expected ErrKeyNotFound")

		// Test Put
		value := []byte("hello world")
		err = database.Put(key, value)
		require.NoError(t, err, "Put operation failed")

		// Test Has after Put
		has, err = database.Has(key)
		require.NoError(t, err, "Has operation failed")
		require.True(t, has, "key should exist after Put")

		// Test Get after Put
		got, err := database.Get(key)
		require.NoError(t, err, "Get operation failed")
		require.Equal(t, value, got, "Get returned wrong value")

		// Test Delete
		err = database.Delete(key)
		require.NoError(t, err, "Delete operation failed")

		// Test Has after Delete
		has, err = database.Has(key)
		require.NoError(t, err, "Has operation failed")
		require.False(t, has, "key should not exist after Delete")

		// Test Get after Delete
		_, err = database.Get(key)
		require.Error(t, err, "Get should fail after Delete")
		require.Equal(t, ErrKeyNotFound, err, "expected ErrKeyNotFound")
	})

	t.Run("Batch", func(t *testing.T) {
		database := newDB()
		defer database.Close()

		// Create a batch and add operations
		batch := database.NewBatch()
		keys := []string{"1", "2", "3", "4"}
		for _, k := range keys {
			err := batch.Put([]byte(k), []byte("value-"+k))
			require.NoError(t, err, "batch Put failed")
		}

		// Verify database doesn't contain keys before Write
		for _, k := range keys {
			has, err := database.Has([]byte(k))
			require.NoError(t, err, "Has operation failed")
			require.False(t, has, "database should not contain key before batch Write")
		}

		// Write the batch
		err := batch.Write()
		require.NoError(t, err, "batch Write failed")

		// Verify database contains keys after Write
		for _, k := range keys {
			has, err := database.Has([]byte(k))
			require.NoError(t, err, "Has operation failed")
			require.True(t, has, "database should contain key after batch Write")

			val, err := database.Get([]byte(k))
			require.NoError(t, err, "Get operation failed")
			require.Equal(t, []byte("value-"+k), val, "wrong value after batch Write")
		}

		// Reset the batch
		batch.Reset()

		// Mix writes and deletes in batch
		err = batch.Put([]byte("5"), []byte("value-5"))
		require.NoError(t, err, "batch Put failed")

		err = batch.Delete([]byte("1"))
		require.NoError(t, err, "batch Delete failed")

		err = batch.Put([]byte("6"), []byte("value-6"))
		require.NoError(t, err, "batch Put failed")

		// Delete then put (put should win)
		err = batch.Delete([]byte("3"))
		require.NoError(t, err, "batch Delete failed")

		err = batch.Put([]byte("3"), []byte("new-value-3"))
		require.NoError(t, err, "batch Put failed")

		// Put then delete (delete should win)
		err = batch.Put([]byte("7"), []byte("value-7"))
		require.NoError(t, err, "batch Put failed")

		err = batch.Delete([]byte("7"))
		require.NoError(t, err, "batch Delete failed")

		// Write the batch
		err = batch.Write()
		require.NoError(t, err, "batch Write failed")

		// Verify expected state after second batch
		expectedPresent := map[string]string{
			"2": "value-2",
			"3": "new-value-3",
			"4": "value-4",
			"5": "value-5",
			"6": "value-6",
		}
		expectedAbsent := []string{"1", "7"}

		// Check present keys
		for k, expectedVal := range expectedPresent {
			has, err := database.Has([]byte(k))
			require.NoError(t, err, "Has operation failed")
			require.True(t, has, "key %s should be present", k)

			val, err := database.Get([]byte(k))
			require.NoError(t, err, "Get operation failed")
			require.Equal(t, []byte(expectedVal), val, "wrong value for key %s", k)
		}

		// Check absent keys
		for _, k := range expectedAbsent {
			has, err := database.Has([]byte(k))
			require.NoError(t, err, "Has operation failed")
			require.False(t, has, "key %s should be absent", k)
		}
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
				err := database.Put([]byte(strconv.Itoa(i)), []byte("value-"+strconv.Itoa(i)))
				require.NoError(t, err, "Put operation failed")
			}
		}

		// Helper to check if a range of keys exists
		checkRange := func(start, stop int, expected bool) {
			for i := start; i <= stop; i++ {
				key := []byte(strconv.Itoa(i))
				has, err := database.Has(key)
				require.NoError(t, err, "Has operation failed")
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
			require.NoError(t, err, "Put operation failed")
		}

		// Create a snapshot
		snapshot := database.NewSnapshot()
		// Note: Snapshot doesn't have a Close method in the interface

		// Modify the database after snapshot
		err := database.Put([]byte("key4"), []byte("value4"))
		require.NoError(t, err, "Put operation failed")

		err = database.Delete([]byte("key1"))
		require.NoError(t, err, "Delete operation failed")

		err = database.Put([]byte("key2"), []byte("modified-value2"))
		require.NoError(t, err, "Put operation failed")

		// Check snapshot has original data
		for k, v := range initialData {
			val, err := snapshot.Get([]byte(k))
			require.NoError(t, err, "Get from snapshot failed")
			require.Equal(t, []byte(v), val, "snapshot value mismatch")
		}

		// Snapshot should not have new key
		_, err = snapshot.Get([]byte("key4"))
		require.Error(t, err, "key4 should not exist in snapshot")
		require.Equal(t, ErrKeyNotFound, err, "expected ErrKeyNotFound")

		// Check database has modified data
		// key1 should be deleted
		has, err := database.Has([]byte("key1"))
		require.NoError(t, err, "Has operation failed")
		require.False(t, has, "key1 should be deleted in database")

		// key2 should have modified value
		val, err := database.Get([]byte("key2"))
		require.NoError(t, err, "Get operation failed")
		require.Equal(t, []byte("modified-value2"), val, "key2 should have modified value")

		// key4 should exist
		val, err = database.Get([]byte("key4"))
		require.NoError(t, err, "Get operation failed")
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
		err := database.Put([]byte("key"), []byte("value"))
		require.NoError(t, err, "Put operation failed")

		// Close the database
		err = database.Close()
		require.NoError(t, err, "Close operation failed")

		// Operations should fail after close
		_, err = database.Get([]byte("key"))
		require.Error(t, err, "Get should fail after Close")

		_, err = database.Has([]byte("key"))
		require.Error(t, err, "Has should fail after Close")

		err = database.Put([]byte("key2"), []byte("value2"))
		require.Error(t, err, "Put should fail after Close")

		err = database.Delete([]byte("key"))
		require.Error(t, err, "Delete should fail after Close")

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
	})
}
