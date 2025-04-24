package hashdb

import (
	"sync"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func verifyNode(t *testing.T, database *Database, id trieutils.TrieID, path trieutils.Path, node trienode.TrieNode) {
	t.Helper()

	reader, err := database.NodeReader(id)
	require.NoError(t, err)

	blob, err := reader.Node(id.Owner(), path, node.Hash(), node.IsLeaf())
	require.NoError(t, err)
	assert.Equal(t, node.Blob(), blob)

	// Verify node is not in dirty cache
	key := trieutils.NodeKeyByHash(id.Bucket(), id.Owner(), path, node.Hash(), node.IsLeaf())
	_, found := database.dirtyCache.Get(key)
	assert.False(t, found, "node should not be in dirty cache after commit")
}

func TestDatabase(t *testing.T) {
	t.Run("New creates database with correct defaults", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)
		assert.NotNil(t, database)
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
		assert.NotNil(t, database)
	})

	t.Run("Update and Commit basic trie structure", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		rootPath := trieutils.NewBitArray(8, 0x01)
		rootNode := trienode.NewNonLeaf(*new(felt.Felt).SetUint64(100), []byte{1, 0, 0})

		leaf1Path := trieutils.NewBitArray(8, 0x02)
		leaf1Node := trienode.NewLeaf([]byte{1, 2, 3})

		leaf2Path := trieutils.NewBitArray(8, 0x03)
		leaf2Node := trienode.NewLeaf([]byte{4, 5, 6})

		root := *new(felt.Felt).SetUint64(100)
		parent := *new(felt.Felt).SetUint64(99)
		blockNum := uint64(42)

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:  rootNode,
			leaf1Path: leaf1Node,
			leaf2Path: leaf2Node,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(root, parent, blockNum, classNodes, contractNodes)

		err := database.Commit(root)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf1Path, leaf1Node)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)
	})

	t.Run("Update and Commit deep trie structure", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		rootPath := trieutils.NewBitArray(8, 0x01)
		rootNode := trienode.NewNonLeaf(*new(felt.Felt).SetUint64(100), []byte{1, 0, 0})

		level1Path1 := trieutils.NewBitArray(8, 0x02)
		level1Node1 := trienode.NewNonLeaf(*new(felt.Felt).SetUint64(101), []byte{1, 0, 0})

		level1Path2 := trieutils.NewBitArray(8, 0x03)
		level1Node2 := trienode.NewNonLeaf(*new(felt.Felt).SetUint64(102), []byte{1, 0, 0})

		leaf1Path := trieutils.NewBitArray(8, 0x04)
		leaf1Node := trienode.NewLeaf([]byte{1, 2, 3})

		leaf2Path := trieutils.NewBitArray(8, 0x05)
		leaf2Node := trienode.NewLeaf([]byte{4, 5, 6})

		root := *new(felt.Felt).SetUint64(100)
		parent := *new(felt.Felt).SetUint64(99)
		blockNum := uint64(42)

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:    rootNode,
			level1Path1: level1Node1,
			level1Path2: level1Node2,
			leaf1Path:   leaf1Node,
			leaf2Path:   leaf2Node,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(root, parent, blockNum, classNodes, contractNodes)

		err := database.Commit(root)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), level1Path1, level1Node1)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), level1Path2, level1Node2)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf1Path, leaf1Node)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)
	})

	t.Run("Update and Commit with contract nodes", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		classPath := trieutils.NewBitArray(8, 0xAA)
		classNode := trienode.NewLeaf([]byte{1, 2, 3})

		contractOwner := *new(felt.Felt).SetUint64(123)
		contractPath := trieutils.NewBitArray(8, 0xBB)
		contractNode := trienode.NewLeaf([]byte{4, 5, 6})

		root := *new(felt.Felt).SetUint64(100)
		parent := *new(felt.Felt).SetUint64(99)
		blockNum := uint64(42)

		classNodes := map[trieutils.Path]trienode.TrieNode{
			classPath: classNode,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{
			contractOwner: {
				contractPath: contractNode,
			},
		}

		database.Update(root, parent, blockNum, classNodes, contractNodes)

		err := database.Commit(root)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), classPath, classNode)
		verifyNode(t, database, trieutils.NewContractStorageTrieID(felt.Zero, contractOwner), contractPath, contractNode)
	})

	t.Run("Commit handles concurrent operations", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		numTries := 5
		tries := make([]struct {
			root       felt.Felt
			parent     felt.Felt
			classNodes map[trieutils.Path]trienode.TrieNode
		}, numTries)

		for i := range numTries {
			rootPath := trieutils.NewBitArray(8, uint64(i*2))
			rootNode := trienode.NewNonLeaf(*new(felt.Felt).SetUint64(uint64(i * 100)), []byte{1, 0, 0})

			leafPath := trieutils.NewBitArray(8, uint64(i*2+1))
			leafNode := trienode.NewLeaf([]byte{byte(i), byte(i + 1), byte(i + 2)})

			tries[i] = struct {
				root       felt.Felt
				parent     felt.Felt
				classNodes map[trieutils.Path]trienode.TrieNode
			}{
				root:   *new(felt.Felt).SetUint64(uint64(i * 100)),
				parent: *new(felt.Felt).SetUint64(uint64(i*100 - 1)),
				classNodes: map[trieutils.Path]trienode.TrieNode{
					rootPath: rootNode,
					leafPath: leafNode,
				},
			}

			database.Update(tries[i].root, tries[i].parent, uint64(i), tries[i].classNodes, nil)
		}

		var wg sync.WaitGroup
		wg.Add(numTries)
		for i := range numTries {
			go func(i int) {
				defer wg.Done()
				err := database.Commit(tries[i].root)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		for _, trie := range tries {
			for path, node := range trie.classNodes {
				verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), path, node)
			}
		}
	})

	t.Run("Update and Commit with deleted nodes", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		rootPath := trieutils.NewBitArray(8, 0x01)
		rootNode := trienode.NewNonLeaf(*new(felt.Felt).SetUint64(100), []byte{1, 0, 0})

		leaf1Path := trieutils.NewBitArray(8, 0x02)
		leaf1Node := trienode.NewLeaf([]byte{1, 2, 3})

		leaf2Path := trieutils.NewBitArray(8, 0x03)
		leaf2Node := trienode.NewLeaf([]byte{4, 5, 6})

		root := *new(felt.Felt).SetUint64(100)
		parent := *new(felt.Felt).SetUint64(99)
		blockNum := uint64(42)

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:  rootNode,
			leaf1Path: leaf1Node,
			leaf2Path: leaf2Node,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(root, parent, blockNum, classNodes, contractNodes)

		err := database.Commit(root)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)

		// Verify deleted node is not in database
		reader, err := database.NodeReader(trieutils.NewClassTrieID(felt.Zero))
		require.NoError(t, err)
		_, err = reader.Node(felt.Zero, leaf1Path, leaf1Node.Hash(), leaf1Node.IsLeaf())
		require.Error(t, err)
	})

	t.Run("Cap empty cache", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		err := database.Cap(1000)
		require.NoError(t, err)
	})

	t.Run("Cap cache with nodes", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		rootPath := trieutils.NewBitArray(8, 0x01)
		rootNode := trienode.NewNonLeaf(*new(felt.Felt).SetUint64(100), []byte{1, 0, 0})

		leaf1Path := trieutils.NewBitArray(8, 0x02)
		leaf1Node := trienode.NewLeaf([]byte{1, 2, 3})

		leaf2Path := trieutils.NewBitArray(8, 0x03)
		leaf2Node := trienode.NewLeaf([]byte{4, 5, 6})

		root := *new(felt.Felt).SetUint64(100)
		parent := *new(felt.Felt).SetUint64(99)
		blockNum := uint64(42)

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:  rootNode,
			leaf1Path: leaf1Node,
			leaf2Path: leaf2Node,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(root, parent, blockNum, classNodes, contractNodes)

		initialSize := database.dirtyCacheSize
		err := database.Cap(uint64(initialSize / 2))
		require.NoError(t, err)

		assert.Less(t, database.dirtyCacheSize, initialSize)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf1Path, leaf1Node)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)
	})

	t.Run("Cap to size larger than cache", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		rootPath := trieutils.NewBitArray(8, 0x01)
		rootNode := trienode.NewNonLeaf(*new(felt.Felt).SetUint64(100), []byte{1, 0, 0})

		root := *new(felt.Felt).SetUint64(100)
		parent := *new(felt.Felt).SetUint64(99)
		blockNum := uint64(42)

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath: rootNode,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(root, parent, blockNum, classNodes, contractNodes)

		initialSize := database.dirtyCacheSize
		err := database.Cap(uint64(initialSize * 2))
		require.NoError(t, err)

		assert.Equal(t, initialSize, database.dirtyCacheSize)
	})
}
