package hashdb

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabase(t *testing.T) {
	t.Run("New creates database with correct defaults", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		assert.Equal(t, memDB, database.disk)
		assert.Equal(t, DefaultConfig, database.config)
		assert.NotNil(t, database.CleanCache)
		assert.NotNil(t, database.DirtyCache)
	})

	t.Run("New creates database with provided config", func(t *testing.T) {
		memDB := memory.New()
		config := &Config{
			DirtyCacheSize: 1024,
			CleanCacheSize: 1024,
			DirtyCacheType: CacheTypeLRU,
			CleanCacheType: CacheTypeFastCache,
		}
		database := New(memDB, config)

		assert.Equal(t, memDB, database.disk)
		assert.Equal(t, config, database.config)
		assert.NotNil(t, database.CleanCache)
		assert.NotNil(t, database.DirtyCache)
	})

	t.Run("insert adds node to dirty cache", func(t *testing.T) {
		database := New(memory.New(), nil)
		bucket := db.ContractStorage
		owner := felt.Zero
		path := trieutils.NewBitArray(8, 0xAA)
		hash := *new(felt.Felt).SetUint64(123)
		blob := []byte{1, 2, 3}
		isLeaf := true

		err := database.insert(bucket, owner, path, hash, blob, isLeaf)
		require.NoError(t, err)

		key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
		cachedNode, found := database.DirtyCache.Get(key)

		assert.True(t, found)
		assert.Equal(t, blob, cachedNode.blob)
		assert.Equal(t, uint32(0), cachedNode.parents)
		assert.Empty(t, cachedNode.external)
		assert.Equal(t, len(blob)+database.hashLen(), database.dirtyCacheSize)
	})

	t.Run("Node retrieves from caches and disk", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)
		bucket := db.ContractStorage
		owner := felt.Zero
		path := trieutils.NewBitArray(8, 0xAA)
		hash := *new(felt.Felt).SetUint64(123)
		blob := []byte{1, 2, 3}
		isLeaf := true
		key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)

		t.Run("retrieves from clean cache", func(t *testing.T) {
			database.CleanCache.Set(key, blob)

			result, err := database.Node(bucket, owner, path, hash, isLeaf)
			require.NoError(t, err)
			assert.Equal(t, blob, result)
		})

		t.Run("retrieves from dirty cache", func(t *testing.T) {
			database := New(memDB, nil)
			err := database.insert(bucket, owner, path, hash, blob, isLeaf)
			require.NoError(t, err)

			result, err := database.Node(bucket, owner, path, hash, isLeaf)
			require.NoError(t, err)
			assert.Equal(t, blob, result)
		})

		t.Run("retrieves from disk and adds to clean cache", func(t *testing.T) {
			database := New(memDB, nil)

			err := memDB.Put(key, blob)
			require.NoError(t, err)

			_, found := database.CleanCache.Get(key)
			assert.False(t, found)

			result, err := database.Node(bucket, owner, path, hash, isLeaf)
			require.NoError(t, err)
			assert.Equal(t, blob, result)

			cachedBlob, found := database.CleanCache.Get(key)
			assert.True(t, found)
			assert.Equal(t, blob, cachedBlob)
		})

		t.Run("returns error when node not found", func(t *testing.T) {
			database := New(memory.New(), nil)

			nonExistentHash := *new(felt.Felt).SetUint64(456)
			_, err := database.Node(bucket, owner, path, nonExistentHash, isLeaf)
			require.Error(t, err)
		})
	})

	t.Run("remove deletes node from dirty cache", func(t *testing.T) {
		database := New(memory.New(), nil)
		bucket := db.ContractStorage
		owner := felt.Zero
		path := trieutils.NewBitArray(8, 0xAA)
		hash := *new(felt.Felt).SetUint64(123)
		blob := []byte{1, 2, 3}
		isLeaf := true

		err := database.insert(bucket, owner, path, hash, blob, isLeaf)
		require.NoError(t, err)

		key := trieutils.NodeKeyByHash(bucket, owner, path, hash, isLeaf)
		_, found := database.DirtyCache.Get(key)
		assert.True(t, found)

		err = database.remove(bucket, owner, path, hash, blob, isLeaf)
		require.NoError(t, err)

		_, found = database.DirtyCache.Get(key)
		assert.False(t, found)
		assert.Equal(t, 0, database.dirtyCacheSize)
	})

	t.Run("Cap flushes dirty cache to disk", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)
		bucket := db.ContractStorage
		owner := felt.Zero

		for i := uint64(0); i < 5; i++ {
			path := trieutils.NewBitArray(8, i)
			hash := *new(felt.Felt).SetUint64(i)
			blob := []byte{byte(i), byte(i + 1), byte(i + 2)}

			err := database.insert(bucket, owner, path, hash, blob, false)
			require.NoError(t, err)
		}

		initialCacheSize := database.dirtyCacheSize
		assert.Greater(t, initialCacheSize, 0)

		err := database.Cap(0)
		require.NoError(t, err)

		assert.Equal(t, 0, database.dirtyCacheSize)
		assert.Equal(t, 0, database.DirtyCache.Len())

		for i := uint64(0); i < 5; i++ {
			path := trieutils.NewBitArray(8, i)
			hash := *new(felt.Felt).SetUint64(i)
			key := trieutils.NodeKeyByHash(bucket, owner, path, hash, false)

			has, err := memDB.Has(key)
			require.NoError(t, err)
			assert.True(t, has)
		}
	})

	t.Run("Cap flushes until limit is reached", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)
		bucket := db.ContractStorage
		owner := felt.Zero

		nodeKeys := make([][]byte, 5)
		var totalSize int
		for i := uint64(0); i < 5; i++ {
			path := trieutils.NewBitArray(8, i)
			hash := *new(felt.Felt).SetUint64(i)
			blob := []byte{byte(i), byte(i + 1), byte(i + 2)}

			err := database.insert(bucket, owner, path, hash, blob, false)
			require.NoError(t, err)

			nodeKeys[i] = trieutils.NodeKeyByHash(bucket, owner, path, hash, false)
			totalSize += len(blob) + database.hashLen()
		}

		limit := uint64(totalSize / 2)

		err := database.Cap(limit)
		require.NoError(t, err)

		assert.Greater(t, database.dirtyCacheSize, 0)
		assert.Greater(t, database.DirtyCache.Len(), 0)

		diskCount := 0
		for i := 0; i < len(nodeKeys); i++ {
			has, err := memDB.Has(nodeKeys[i])
			require.NoError(t, err)
			if has {
				diskCount++
			}
		}
		assert.Greater(t, diskCount, 0)
	})

	t.Run("Update adds nodes to cache", func(t *testing.T) {
		database := New(memory.New(), nil)

		classPath := trieutils.NewBitArray(8, 0xAA)
		classNode := trienode.NewLeaf([]byte{1, 2, 3})

		contractOwner := *new(felt.Felt).SetUint64(123)
		contractPath := trieutils.NewBitArray(8, 0xBB)
		contractNode := trienode.NewLeaf([]byte{4, 5, 6})

		classNodes := map[trieutils.Path]trienode.TrieNode{
			classPath: classNode,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{
			contractOwner: {
				contractPath: contractNode,
			},
		}

		root := *new(felt.Felt).SetUint64(999)
		parent := *new(felt.Felt).SetUint64(888)
		blockNum := uint64(42)

		database.Update(root, parent, blockNum, classNodes, contractNodes)

		classKey := trieutils.NodeKeyByHash(db.ContractStorage, felt.Zero, classPath, classNode.Hash(), classNode.IsLeaf())
		classResult, found := database.DirtyCache.Get(classKey)
		assert.True(t, found)
		assert.Equal(t, classNode.Blob(), classResult.blob)

		contractKey := trieutils.NodeKeyByHash(db.ContractTrieStorage, contractOwner, contractPath, contractNode.Hash(), contractNode.IsLeaf())
		contractResult, found := database.DirtyCache.Get(contractKey)
		assert.True(t, found)
		assert.Equal(t, contractNode.Blob(), contractResult.blob)
	})

	t.Run("Update deletes nodes marked for deletion", func(t *testing.T) {
		database := New(memory.New(), nil)

		bucket := db.ContractStorage
		owner := felt.Zero
		path := trieutils.NewBitArray(8, 0xAA)
		hash := *new(felt.Felt).SetUint64(123)
		blob := []byte{1, 2, 3}

		err := database.insert(bucket, owner, path, hash, blob, false)
		require.NoError(t, err)

		deletedNode := trienode.NewDeleted(false)

		classNodes := map[trieutils.Path]trienode.TrieNode{
			path: deletedNode,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		root := *new(felt.Felt).SetUint64(999)
		parent := *new(felt.Felt).SetUint64(888)
		blockNum := uint64(42)

		database.Update(root, parent, blockNum, classNodes, contractNodes)

		key := trieutils.NodeKeyByHash(bucket, owner, path, hash, false)
		_, found := database.DirtyCache.Get(key)
		assert.False(t, found)
	})

	t.Run("Commit adds nodes to disk and moves from dirty to clean cache", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)
		bucket := db.ContractStorage
		owner := felt.Zero

		rootPath := trieutils.NewBitArray(8, 0x01)
		rootHash := *new(felt.Felt).SetUint64(100)
		rootBlob := []byte{10, 20, 30}
		rootIsLeaf := false
		rootKey := trieutils.NodeKeyByHash(bucket, owner, rootPath, rootHash, rootIsLeaf)

		err := database.insert(bucket, owner, rootPath, rootHash, rootBlob, rootIsLeaf)
		require.NoError(t, err)

		childPath1 := trieutils.NewBitArray(8, 0x02)
		childHash1 := *new(felt.Felt).SetUint64(101)
		childBlob1 := []byte{11, 21, 31}
		childIsLeaf1 := true
		childKey1 := trieutils.NodeKeyByHash(bucket, owner, childPath1, childHash1, childIsLeaf1)

		childPath2 := trieutils.NewBitArray(8, 0x03)
		childHash2 := *new(felt.Felt).SetUint64(102)
		childBlob2 := []byte{12, 22, 32}
		childIsLeaf2 := true
		childKey2 := trieutils.NodeKeyByHash(bucket, owner, childPath2, childHash2, childIsLeaf2)

		err = database.insert(bucket, owner, childPath1, childHash1, childBlob1, childIsLeaf1)
		require.NoError(t, err)
		err = database.insert(bucket, owner, childPath2, childHash2, childBlob2, childIsLeaf2)
		require.NoError(t, err)

		rootNode, found := database.DirtyCache.Get(rootKey)
		require.True(t, found)

		rootNode.external = map[string]struct{}{
			string(childKey1): {},
			string(childKey2): {},
		}
		database.DirtyCache.Set(rootKey, rootNode)

		err = database.Commit(rootHash)
		require.NoError(t, err)

		_, found = database.DirtyCache.Get(rootKey)
		assert.False(t, found)

		_, found = database.DirtyCache.Get(childKey1)
		assert.False(t, found)

		_, found = database.DirtyCache.Get(childKey2)
		assert.False(t, found)

		cachedRootBlob, found := database.CleanCache.Get(rootKey)
		assert.True(t, found)
		assert.Equal(t, rootBlob, cachedRootBlob)

		cachedChildBlob1, found := database.CleanCache.Get(childKey1)
		assert.True(t, found)
		assert.Equal(t, childBlob1, cachedChildBlob1)

		cachedChildBlob2, found := database.CleanCache.Get(childKey2)
		assert.True(t, found)
		assert.Equal(t, childBlob2, cachedChildBlob2)

		var diskRootBlob []byte
		err = memDB.Get(rootKey, func(val []byte) error {
			diskRootBlob = val
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, rootBlob, diskRootBlob)

		var diskChildBlob1 []byte
		err = memDB.Get(childKey1, func(val []byte) error {
			diskChildBlob1 = val
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, childBlob1, diskChildBlob1)

		var diskChildBlob2 []byte
		err = memDB.Get(childKey2, func(val []byte) error {
			diskChildBlob2 = val
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, childBlob2, diskChildBlob2)
	})

	t.Run("NewIterator creates iterator with correct prefix", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		owner := *new(felt.Felt).SetUint64(123)
		mockTrieID := trieutils.NewClassTrieID(felt.Zero)

		bucket := mockTrieID.Bucket()
		for i := uint64(0); i < 3; i++ {
			path := trieutils.NewBitArray(8, i)
			hash := *new(felt.Felt).SetUint64(i)
			blob := []byte{byte(i), byte(i + 1), byte(i + 2)}
			key := trieutils.NodeKeyByHash(bucket, owner, path, hash, false)
			err := memDB.Put(key, blob)
			require.NoError(t, err)
		}

		iterator, err := database.NewIterator(mockTrieID)
		require.NoError(t, err)
		defer iterator.Close()

		count := 0
		for iterator.Next() {
			count++
			key := iterator.Key()
			assert.True(t, len(key) > 0)

			assert.Equal(t, byte(bucket), key[0])

			value, err := iterator.Value()
			require.NoError(t, err)
			assert.True(t, len(value) > 0)
		}

		assert.Equal(t, 3, count)
	})

	t.Run("NodeReader interface", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		mockTrieID := trieutils.NewClassTrieID(felt.Zero)

		path := trieutils.NewBitArray(8, 0xAA)
		hash := *new(felt.Felt).SetUint64(123)
		blob := []byte{1, 2, 3}
		isLeaf := true

		key := trieutils.NodeKeyByHash(mockTrieID.Bucket(), mockTrieID.Owner(), path, hash, isLeaf)
		err := memDB.Put(key, blob)
		require.NoError(t, err)

		reader, err := database.NodeReader(mockTrieID)
		require.NoError(t, err)

		result, err := reader.Node(mockTrieID.Owner(), path, hash, isLeaf)
		require.NoError(t, err)
		assert.Equal(t, blob, result)
	})
}
