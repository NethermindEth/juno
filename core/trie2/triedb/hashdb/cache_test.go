package hashdb_test

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/triedb/hashdb"
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
	config := hashdb.DefaultConfig

	t.Run("Get from clean cache", func(t *testing.T) {
		db := hashdb.New(txn, prefix, config)
		owner := felt.Felt{}
		owner.SetUint64(123)

		path := trieutils.Path{}
		path.AppendBit(&path, 1)
		path.AppendBit(&path, 0)

		hash := felt.Felt{}
		hash.SetUint64(456)

		data := []byte("test data")

		dbBuf := new(bytes.Buffer)
		hashdb.GetDbKey(t, db, dbBuf, owner, path, hash, true)
		require.NoError(t, txn.Set(dbBuf.Bytes(), data))

		buf := new(bytes.Buffer)
		n, err := db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
		assert.Equal(t, uint64(0), db.CleanCache.Hits())
		buf.Reset()
		n, err = db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
		assert.Equal(t, uint64(1), db.CleanCache.Hits())
	})

	t.Run("Get from dirty cache", func(t *testing.T) {
		db := hashdb.New(txn, prefix, config)
		owner := felt.Felt{}
		owner.SetUint64(123)

		path := trieutils.Path{}
		path.AppendBit(&path, 1)
		path.AppendBit(&path, 0)

		hash := felt.Felt{}
		hash.SetUint64(456)

		data := []byte("test data")

		err = db.Put(owner, path, hash, data, true)
		require.NoError(t, err)

		buf := new(bytes.Buffer)
		n, err := db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
		assert.Equal(t, uint64(1), db.DirtyCache.Hits())
	})

	t.Run("Cache behavior with trie commit", func(t *testing.T) {
		memDB := pebble.NewMemTest(t)
		txn, err := memDB.NewTransaction(true)
		require.NoError(t, err)
		defer func() {
			err = txn.Discard()
			require.NoError(t, err)
		}()

		db := hashdb.New(txn, db.ClassesTrie, hashdb.DefaultConfig)
		trie, err := trie2.New(trie2.NewEmptyTrieID(), 251, crypto.Pedersen, txn)
		require.NoError(t, err)

		owner := felt.Felt{}
		owner.SetUint64(123)

		path := trieutils.Path{}
		path.AppendBit(&path, 1)
		path.AppendBit(&path, 0)

		hash := felt.Felt{}
		hash.SetUint64(456)

		data := []byte("test data")

		err = db.Put(owner, path, hash, data, true)
		require.NoError(t, err)

		buf := new(bytes.Buffer)
		n, err := db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
		assert.Equal(t, uint64(1), db.DirtyCache.Hits())

		_, err = trie.Commit()
		require.NoError(t, err)

		buf.Reset()
		n, err = db.Get(buf, owner, path, hash, true)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, buf.Bytes())
		assert.Equal(t, uint64(2), db.DirtyCache.Hits())
		assert.Equal(t, uint64(0), db.CleanCache.Hits())
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
		config *hashdb.Config
	}{
		{
			name: "LRU Cache",
			config: &hashdb.Config{
				CleanCacheType: hashdb.CacheTypeLRU,
				CleanCacheSize: 1024,
				DirtyCacheType: hashdb.CacheTypeLRU,
				DirtyCacheSize: 1024,
			},
		},
		{
			name: "FastCache",
			config: &hashdb.Config{
				CleanCacheType: hashdb.CacheTypeFastCache,
				CleanCacheSize: 1024 * 1024,
				DirtyCacheType: hashdb.CacheTypeFastCache,
				DirtyCacheSize: 1024,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := hashdb.New(txn, prefix, tc.config)

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
			assert.Equal(t, uint64(0), db.CleanCache.Hits())
			assert.Equal(t, uint64(1), db.DirtyCache.Hits())

			_, err = db.Get(buf, owner, path, hash, true)
			require.NoError(t, err)
			assert.Equal(t, uint64(0), db.CleanCache.Hits())
			assert.Equal(t, uint64(2), db.DirtyCache.Hits())
		})
	}
}
