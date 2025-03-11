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

func GetDbKey(t *testing.T, db *Database, buf *bytes.Buffer, owner felt.Felt, path trieutils.Path, hash felt.Felt, isLeaf bool) {
	err := db.dbKey(buf, owner, path, hash, isLeaf)
	require.NoError(t, err)
}

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
		GetDbKey(t, db, buf, owner, path, hash, true)

		keyBytes := buf.Bytes()
		assert.Equal(t, prefix.Key()[0], keyBytes[0])

		buf.Reset()
		GetDbKey(t, db, buf, owner, path, hash, true)

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
