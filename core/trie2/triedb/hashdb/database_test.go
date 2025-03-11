package hashdb

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabase(t *testing.T) {
	memDB := pebble.NewMemTest(t)
	txn, err := memDB.NewTransaction(true)
	require.NoError(t, err)
	defer txn.Discard()

	prefix := db.ClassesTrie
	config := DefaultConfig

	t.Run("New", func(t *testing.T) {
		db := New(txn, prefix, config)
		assert.NotNil(t, db)
		assert.Equal(t, txn, db.txn)
		assert.Equal(t, prefix, db.prefix)
		assert.Equal(t, config, db.config)
		assert.NotNil(t, db.CleanCache)
		assert.NotNil(t, db.DirtyCache)
	})

	t.Run("New with nil config", func(t *testing.T) {
		db := New(txn, prefix, nil)
		assert.NotNil(t, db)
		assert.Equal(t, DefaultConfig, db.config)
	})

	t.Run("Put and Get", func(t *testing.T) {
		db := New(txn, prefix, config)
		owner := felt.Felt{}
		owner.SetUint64(123)

		path := trieutils.Path{}
		path.AppendBit(&path, 1)
		path.AppendBit(&path, 0)

		hash := felt.Felt{}
		hash.SetUint64(456)

		data := []byte("test data")

		// Test Put
		err := db.Put(owner, path, hash, data, true)
		require.NoError(t, err)

		// Test Get
		buf := new(bytes.Buffer)
		n, err := db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
	})

	t.Run("Get from clean cache", func(t *testing.T) {
		db := New(txn, prefix, config)
		owner := felt.Felt{}
		owner.SetUint64(123)

		path := trieutils.Path{}
		path.AppendBit(&path, 1)
		path.AppendBit(&path, 0)

		hash := felt.Felt{}
		hash.SetUint64(456)

		data := []byte("test data")

		// Put data
		err := db.Put(owner, path, hash, data, true)
		require.NoError(t, err)

		// Get data to populate clean cache
		buf := new(bytes.Buffer)
		_, err = db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)

		// Get again, should hit clean cache
		buf.Reset()
		n, err := db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
	})

	t.Run("Get from dirty cache", func(t *testing.T) {
		db := New(txn, prefix, config)
		owner := felt.Felt{}
		owner.SetUint64(123)

		path := trieutils.Path{}
		path.AppendBit(&path, 1)
		path.AppendBit(&path, 0)

		hash := felt.Felt{}
		hash.SetUint64(456)

		data := []byte("test data")

		// Create a key in the buffer
		dbBuf := dbBufferPool.Get().(*bytes.Buffer)
		dbBuf.Reset()
		defer func() {
			dbBuf.Reset()
			dbBufferPool.Put(dbBuf)
		}()

		err := db.DbKey(dbBuf, owner, path, hash, true)
		require.NoError(t, err)

		// Set directly in dirty cache
		db.dirtyCache.Set(dbBuf.Bytes(), data)

		// Get should hit dirty cache
		buf := new(bytes.Buffer)
		n, err := db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
	})

	t.Run("Delete", func(t *testing.T) {
		db := New(txn, prefix, config)
		owner := felt.Felt{}
		owner.SetUint64(123)

		path := trieutils.Path{}
		path.AppendBit(&path, 1)
		path.AppendBit(&path, 0)

		hash := felt.Felt{}
		hash.SetUint64(456)

		data := []byte("test data")

		// Put data
		err := db.Put(owner, path, hash, data, true)
		require.NoError(t, err)

		// Delete data
		err = db.Delete(owner, path, hash, true)
		require.NoError(t, err)

		// Try to get deleted data
		buf := new(bytes.Buffer)
		_, err = db.Get(buf, owner, path, hash, true)
		assert.Error(t, err)
		assert.Equal(t, 0, buf.Len())
	})

	t.Run("NewIterator", func(t *testing.T) {
		db := New(txn, prefix, config)
		owner := felt.Felt{}
		owner.SetUint64(123)

		// Test with owner
		iter, err := db.NewIterator(owner)
		require.NoError(t, err)
		defer iter.Close()
		assert.NotNil(t, iter)

		// Test with empty owner
		iter, err = db.NewIterator(felt.Felt{})
		require.NoError(t, err)
		defer iter.Close()
		assert.NotNil(t, iter)
	})

	t.Run("DbKey format", func(t *testing.T) {
		db := New(txn, prefix, config)
		owner := felt.Felt{}
		owner.SetUint64(123)

		path := trieutils.Path{}
		path.AppendBit(&path, 1)
		path.AppendBit(&path, 0)

		hash := felt.Felt{}
		hash.SetUint64(456)

		// Test leaf node key
		buf := new(bytes.Buffer)
		err := db.DbKey(buf, owner, path, hash, true)
		require.NoError(t, err)

		// Verify key format
		keyBytes := buf.Bytes()
		assert.Equal(t, prefix.Key()[0], keyBytes[0]) // Prefix

		// Test non-leaf node key
		buf.Reset()
		err = db.DbKey(buf, owner, path, hash, false)
		require.NoError(t, err)

		// Verify key format
		keyBytes = buf.Bytes()
		assert.Equal(t, prefix.Key()[0], keyBytes[0]) // Prefix
	})

	t.Run("EmptyDatabase", func(t *testing.T) {
		emptyDB := EmptyDatabase{}

		// Test Get
		buf := new(bytes.Buffer)
		_, err := emptyDB.Get(buf, felt.Felt{}, trieutils.Path{}, false)
		assert.Equal(t, ErrCallEmptyDatabase, err)

		// Test Put
		err = emptyDB.Put(felt.Felt{}, trieutils.Path{}, []byte{}, false)
		assert.Equal(t, ErrCallEmptyDatabase, err)

		// Test Delete
		err = emptyDB.Delete(felt.Felt{}, trieutils.Path{}, false)
		assert.Equal(t, ErrCallEmptyDatabase, err)

		// Test NewIterator
		_, err = emptyDB.NewIterator(felt.Felt{})
		assert.Equal(t, ErrCallEmptyDatabase, err)
	})
}

func TestDatabaseWithDifferentCaches(t *testing.T) {
	memDB := pebble.NewMemTest(t)
	txn, err := memDB.NewTransaction(true)
	require.NoError(t, err)
	defer txn.Discard()

	prefix := db.ClassesTrie

	testCases := []struct {
		name   string
		config *Config
	}{
		{
			name: "LRU Cache",
			config: &Config{
				CleanCacheType: CacheTypeLRU,
				CleanCacheSize: 1024,
				DirtyCacheType: CacheTypeLRU,
				DirtyCacheSize: 1024,
			},
		},
		{
			name: "FastCache",
			config: &Config{
				CleanCacheType: CacheTypeFastCache,
				CleanCacheSize: 1024 * 1024, // 1MB
				DirtyCacheType: CacheTypeFastCache,
				DirtyCacheSize: 1024,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := New(txn, prefix, tc.config)

			owner := felt.Felt{}
			owner.SetUint64(123)

			path := trieutils.Path{}
			path.AppendBit(&path, 1)
			path.AppendBit(&path, 0)

			hash := felt.Felt{}
			hash.SetUint64(456)

			data := []byte("test data")

			// Test Put
			err := db.Put(owner, path, hash, data, true)
			require.NoError(t, err)

			// Test Get
			buf := new(bytes.Buffer)
			n, err := db.Get(buf, owner, path, hash, true)
			require.NoError(t, err)
			assert.Equal(t, len(data), n)
			assert.Equal(t, data, buf.Bytes())
		})
	}
}
