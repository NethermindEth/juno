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

		reader, err := database.NodeReader(trieutils.NewClassTrieID(felt.Zero))
		require.NoError(t, err)

		rootBlob, err := reader.Node(felt.Zero, rootPath, rootNode.Hash(), false)
		require.NoError(t, err)
		assert.Equal(t, rootNode.Blob(), rootBlob)

		leaf1Blob, err := reader.Node(felt.Zero, leaf1Path, leaf1Node.Hash(), true)
		require.NoError(t, err)
		assert.Equal(t, leaf1Node.Blob(), leaf1Blob)

		leaf2Blob, err := reader.Node(felt.Zero, leaf2Path, leaf2Node.Hash(), true)
		require.NoError(t, err)
		assert.Equal(t, leaf2Node.Blob(), leaf2Blob)
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

		reader, err := database.NodeReader(trieutils.NewClassTrieID(felt.Zero))
		require.NoError(t, err)

		verifyNode := func(path trieutils.Path, node trienode.TrieNode) {
			blob, err := reader.Node(felt.Zero, path, node.Hash(), node.IsLeaf())
			require.NoError(t, err)
			assert.Equal(t, node.Blob(), blob)
		}

		verifyNode(rootPath, rootNode)
		verifyNode(level1Path1, level1Node1)
		verifyNode(level1Path2, level1Node2)
		verifyNode(leaf1Path, leaf1Node)
		verifyNode(leaf2Path, leaf2Node)
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

		classReader, err := database.NodeReader(trieutils.NewClassTrieID(felt.Zero))
		require.NoError(t, err)

		contractReader, err := database.NodeReader(trieutils.NewContractTrieID(contractOwner))
		require.NoError(t, err)

		classBlob, err := classReader.Node(felt.Zero, classPath, classNode.Hash(), classNode.IsLeaf())
		require.NoError(t, err)
		assert.Equal(t, classNode.Blob(), classBlob)

		contractBlob, err := contractReader.Node(contractOwner, contractPath, contractNode.Hash(), contractNode.IsLeaf())
		require.NoError(t, err)
		assert.Equal(t, contractNode.Blob(), contractBlob)
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

		for i := 0; i < numTries; i++ {
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
		for i := 0; i < numTries; i++ {
			go func(i int) {
				defer wg.Done()
				err := database.Commit(tries[i].root)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		reader, err := database.NodeReader(trieutils.NewClassTrieID(felt.Zero))
		require.NoError(t, err)

		for _, trie := range tries {
			for path, node := range trie.classNodes {
				blob, err := reader.Node(felt.Zero, path, node.Hash(), node.IsLeaf())
				require.NoError(t, err)
				assert.Equal(t, node.Blob(), blob)
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

		reader, err := database.NodeReader(trieutils.NewClassTrieID(felt.Zero))
		require.NoError(t, err)

		rootBlob, err := reader.Node(felt.Zero, rootPath, rootNode.Hash(), false)
		require.NoError(t, err)
		assert.Equal(t, rootNode.Blob(), rootBlob)

		leaf1Blob, err := reader.Node(felt.Zero, leaf1Path, leaf1Node.Hash(), true)
		require.NoError(t, err)
		assert.Equal(t, leaf1Node.Blob(), leaf1Blob)

		leaf2Blob, err := reader.Node(felt.Zero, leaf2Path, leaf2Node.Hash(), true)
		require.NoError(t, err)
		assert.Equal(t, leaf2Node.Blob(), leaf2Blob)

		classNodes = map[trieutils.Path]trienode.TrieNode{
			rootPath:  rootNode,
			leaf1Path: trienode.NewDeleted(true),
		}

		database.Update(root, parent, blockNum+1, classNodes, contractNodes)

		err = database.Commit(root)
		require.NoError(t, err)

		rootBlob, err = reader.Node(felt.Zero, rootPath, rootNode.Hash(), false)
		require.NoError(t, err)
		assert.Equal(t, rootNode.Blob(), rootBlob)

		leaf1Blob, err = reader.Node(felt.Zero, leaf1Path, leaf1Node.Hash(), true)
		require.Error(t, err)
		assert.Nil(t, leaf1Blob)

		leaf2Blob, err = reader.Node(felt.Zero, leaf2Path, leaf2Node.Hash(), true)
		require.NoError(t, err)
		assert.Equal(t, leaf2Node.Blob(), leaf2Blob)
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

		reader, err := database.NodeReader(trieutils.NewClassTrieID(felt.Zero))
		require.NoError(t, err)

		rootBlob, err := reader.Node(felt.Zero, rootPath, rootNode.Hash(), false)
		require.NoError(t, err)
		assert.Equal(t, rootNode.Blob(), rootBlob)

		leaf1Blob, err := reader.Node(felt.Zero, leaf1Path, leaf1Node.Hash(), true)
		require.NoError(t, err)
		assert.Equal(t, leaf1Node.Blob(), leaf1Blob)

		leaf2Blob, err := reader.Node(felt.Zero, leaf2Path, leaf2Node.Hash(), true)
		require.NoError(t, err)
		assert.Equal(t, leaf2Node.Blob(), leaf2Blob)
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
