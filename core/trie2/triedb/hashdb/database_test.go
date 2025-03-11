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
	defer func() {
		err = txn.Discard()
		require.NoError(t, err)
	}()

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

		err := db.Put(owner, path, hash, data, true)
		require.NoError(t, err)

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

		err := db.Put(owner, path, hash, data, true)
		require.NoError(t, err)

		buf := new(bytes.Buffer)
		bufLen, err := db.Get(buf, owner, path, hash, true)
		require.NotZero(t, bufLen)
		require.NoError(t, err)
		require.Equal(t, 0, db.CleanCache.Hits())

		buf.Reset()
		n, err := db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
		assert.Equal(t, 1, db.CleanCache.Hits())

		buf.Reset()
		n, err = db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, 2, db.CleanCache.Hits())
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

		dbBuf := dbBufferPool.Get().(*bytes.Buffer)
		dbBuf.Reset()
		defer func() {
			dbBuf.Reset()
			dbBufferPool.Put(dbBuf)
		}()

		err = db.Put(owner, path, hash, data, true)
		require.NoError(t, err)

		buf := new(bytes.Buffer)
		n, err := db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
		assert.Equal(t, 1, db.DirtyCache.Hits())
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

		err := db.Put(owner, path, hash, data, true)
		require.NoError(t, err)

		err = db.Delete(owner, path, hash, true)
		require.NoError(t, err)

		buf := new(bytes.Buffer)
		_, err = db.Get(buf, owner, path, hash, true)
		assert.Error(t, err)
		assert.Equal(t, 0, buf.Len())
	})

	t.Run("NewIterator", func(t *testing.T) {
		db := New(txn, prefix, config)
		owner := felt.Felt{}
		owner.SetUint64(123)

		iter, err := db.NewIterator(owner)
		require.NoError(t, err)
		defer iter.Close()
		assert.NotNil(t, iter)

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

		buf := new(bytes.Buffer)
		err := db.dbKey(buf, owner, path, hash, true)
		require.NoError(t, err)

		keyBytes := buf.Bytes()
		assert.Equal(t, prefix.Key()[0], keyBytes[0])

		buf.Reset()
		err = db.dbKey(buf, owner, path, hash, false)
		require.NoError(t, err)

		keyBytes = buf.Bytes()
		assert.Equal(t, prefix.Key()[0], keyBytes[0])
	})

	t.Run("EmptyDatabase", func(t *testing.T) {
		emptyDB := EmptyDatabase{}

		buf := new(bytes.Buffer)
		_, err := emptyDB.Get(buf, felt.Felt{}, trieutils.Path{}, false)
		assert.Equal(t, ErrCallEmptyDatabase, err)

		err = emptyDB.Put(felt.Felt{}, trieutils.Path{}, []byte{}, false)
		assert.Equal(t, ErrCallEmptyDatabase, err)

		err = emptyDB.Delete(felt.Felt{}, trieutils.Path{}, false)
		assert.Equal(t, ErrCallEmptyDatabase, err)

		_, err = emptyDB.NewIterator(felt.Felt{})
		assert.Equal(t, ErrCallEmptyDatabase, err)
	})
}

func TestDatabaseWithDifferentCaches(t *testing.T) {
	memDB := pebble.NewMemTest(t)
	txn, err := memDB.NewTransaction(true)
	require.NoError(t, err)
	defer func() {
		err = txn.Discard()
		require.NoError(t, err)
	}()
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
				CleanCacheSize: 1024 * 1024,
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

			err := db.Put(owner, path, hash, data, true)
			require.NoError(t, err)

			buf := new(bytes.Buffer)
			n, err := db.Get(buf, owner, path, hash, true)
			require.NoError(t, err)
			assert.Equal(t, len(data), n)
			assert.Equal(t, data, buf.Bytes())
			assert.Equal(t, 0, db.CleanCache.Hits())
			assert.Equal(t, 1, db.DirtyCache.Hits())

			n, err = db.Get(buf, owner, path, hash, true)
			require.NoError(t, err)
			assert.Equal(t, 1, db.CleanCache.Hits())
			assert.Equal(t, 1, db.DirtyCache.Hits())
		})
	}
}
