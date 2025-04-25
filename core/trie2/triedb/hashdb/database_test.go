package hashdb

import (
	"sync"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockStoreRootsForStateComm(memDB db.KeyValueStore, stateComm, classRoot, contractRoot felt.Felt) error {
	stateBytes := stateComm.Bytes()
	key := db.StateHashToRoots.Key(stateBytes[:])

	value := make([]byte, 2*felt.Bytes)
	classBytes := classRoot.Bytes()
	contractBytes := contractRoot.Bytes()
	copy(value[:felt.Bytes], classBytes[:])
	copy(value[felt.Bytes:], contractBytes[:])

	batch := memDB.NewBatch()
	if err := batch.Put(key, value); err != nil {
		return err
	}
	return batch.Write()
}

func verifyNode(t *testing.T, database *Database, id trieutils.TrieID, path trieutils.Path, node trienode.TrieNode) {
	t.Helper()

	reader, err := database.NodeReader(id)
	require.NoError(t, err)

	blob, err := reader.Node(id.Owner(), path, node.Hash(), node.IsLeaf())
	require.NoError(t, err)
	assert.Equal(t, node.Blob(), blob)

	key := trieutils.NodeKeyByHash(id.Bucket(), id.Owner(), path, node.Hash(), node.IsLeaf())
	_, found := database.dirtyCache.Get(key)
	assert.False(t, found)
}

func createBinaryNodeBlob(leftHash, rightHash felt.Felt) []byte {
	binaryBlob := make([]byte, 1+2*felt.Bytes)
	binaryBlob[0] = 1

	leftBytes := leftHash.Bytes()
	rightBytes := rightHash.Bytes()

	copy(binaryBlob[1:felt.Bytes+1], leftBytes[:])
	copy(binaryBlob[felt.Bytes+1:], rightBytes[:])

	return binaryBlob
}

func createEdgeNodeBlob(childHash felt.Felt) []byte {
	edgeBlob := make([]byte, 1+felt.Bytes)
	edgeBlob[0] = 2

	childBytes := childHash.Bytes()
	copy(edgeBlob[1:felt.Bytes+1], childBytes[:])

	return edgeBlob
}

func NewLeafWithHash(blob []byte, hash felt.Felt) trienode.TrieNode {
	return &customLeafNode{
		blob: blob,
		hash: hash,
	}
}

type customLeafNode struct {
	blob []byte
	hash felt.Felt
}

func (c *customLeafNode) Blob() []byte    { return c.blob }
func (c *customLeafNode) Hash() felt.Felt { return c.hash }
func (c *customLeafNode) IsLeaf() bool    { return true }

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

		leaf1Hash := *new(felt.Felt).SetUint64(201)
		leaf1Path := trieutils.NewBitArray(1, 0x00)
		leaf1Node := NewLeafWithHash([]byte{1, 2, 3}, leaf1Hash)

		leaf2Hash := *new(felt.Felt).SetUint64(202)
		leaf2Path := trieutils.NewBitArray(1, 0x01)
		leaf2Node := NewLeafWithHash([]byte{4, 5, 6}, leaf2Hash)

		rootHash := *new(felt.Felt).SetUint64(100)
		rootPath := trieutils.NewBitArray(0, 0x0)
		rootNode := trienode.NewNonLeaf(rootHash, createBinaryNodeBlob(leaf1Hash, leaf2Hash))
		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:  rootNode,
			leaf1Path: leaf1Node,
			leaf2Path: leaf2Node,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(felt.Zero, felt.Zero, 42, classNodes, contractNodes)

		classRoot := rootHash
		contractRoot := *new(felt.Felt).SetUint64(102)
		stateCommitment := *new(felt.Felt).SetUint64(103)

		err := mockStoreRootsForStateComm(memDB, stateCommitment, classRoot, contractRoot)
		require.NoError(t, err)

		err = database.Commit(stateCommitment)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf1Path, leaf1Node)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)
	})

	t.Run("Update and Commit deep trie structure", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		leaf1Hash := *new(felt.Felt).SetUint64(301)
		leaf2Hash := *new(felt.Felt).SetUint64(302)
		level1Hash1 := *new(felt.Felt).SetUint64(201)
		level1Hash2 := *new(felt.Felt).SetUint64(202)
		rootHash := *new(felt.Felt).SetUint64(100)

		leaf1Path := trieutils.NewBitArray(2, 0x00)
		leaf1Node := NewLeafWithHash([]byte{1, 2, 3}, leaf1Hash)

		leaf2Path := trieutils.NewBitArray(2, 0x03)
		leaf2Node := NewLeafWithHash([]byte{4, 5, 6}, leaf2Hash)

		level1Path1 := trieutils.NewBitArray(1, 0x00)
		level1Node1 := trienode.NewNonLeaf(level1Hash1, createEdgeNodeBlob(leaf1Hash))

		level1Path2 := trieutils.NewBitArray(1, 0x01)
		level1Node2 := trienode.NewNonLeaf(level1Hash2, createEdgeNodeBlob(leaf2Hash))

		rootPath := trieutils.NewBitArray(0, 0x0)
		rootNode := trienode.NewNonLeaf(rootHash, createBinaryNodeBlob(level1Hash1, level1Hash2))

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:    rootNode,
			level1Path1: level1Node1,
			level1Path2: level1Node2,
			leaf1Path:   leaf1Node,
			leaf2Path:   leaf2Node,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(felt.Zero, felt.Zero, 42, classNodes, contractNodes)

		classRoot := rootHash
		contractRoot := *new(felt.Felt).SetUint64(112)
		stateCommitment := *new(felt.Felt).SetUint64(113)

		err := mockStoreRootsForStateComm(memDB, stateCommitment, classRoot, contractRoot)
		require.NoError(t, err)

		err = database.Commit(stateCommitment)
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

		classHash := *new(felt.Felt).SetUint64(200)
		classPath := trieutils.NewBitArray(1, 0x00)
		classNode := NewLeafWithHash([]byte{1, 2, 3}, classHash)

		contractHash := *new(felt.Felt).SetUint64(210)
		contractOwner := *new(felt.Felt).SetUint64(123)
		contractPath := trieutils.NewBitArray(1, 0x01)
		contractNode := NewLeafWithHash([]byte{4, 5, 6}, contractHash)

		classNodes := map[trieutils.Path]trienode.TrieNode{
			classPath: classNode,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{
			contractOwner: {
				contractPath: contractNode,
			},
		}

		database.Update(felt.Zero, felt.Zero, 42, classNodes, contractNodes)

		classRoot := classHash
		contractRoot := contractHash
		stateCommitment := *new(felt.Felt).SetUint64(123)

		err := mockStoreRootsForStateComm(memDB, stateCommitment, classRoot, contractRoot)
		require.NoError(t, err)

		err = database.Commit(stateCommitment)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), classPath, classNode)
		verifyNode(t, database, trieutils.NewContractStorageTrieID(felt.Zero, contractOwner), contractPath, contractNode)
	})

	t.Run("Update and Commit deep trie structure with edge nodes", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		leafHash := *new(felt.Felt).SetUint64(301)
		edgeHash := *new(felt.Felt).SetUint64(201)
		rootHash := *new(felt.Felt).SetUint64(100)

		leafPath := trieutils.NewBitArray(2, 0x01)
		leafNode := NewLeafWithHash([]byte{1, 2, 3}, leafHash)

		edgePath := trieutils.NewBitArray(1, 0x01)

		edgeNode := trienode.NewNonLeaf(edgeHash, createEdgeNodeBlob(leafHash))

		rootPath := trieutils.NewBitArray(0, 0x0)
		rootNode := trienode.NewNonLeaf(rootHash, createEdgeNodeBlob(edgeHash))

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath: rootNode,
			edgePath: edgeNode,
			leafPath: leafNode,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(felt.Zero, felt.Zero, 42, classNodes, contractNodes)

		classRoot := rootHash
		contractRoot := *new(felt.Felt).SetUint64(112)
		stateCommitment := *new(felt.Felt).SetUint64(113)

		err := mockStoreRootsForStateComm(memDB, stateCommitment, classRoot, contractRoot)
		require.NoError(t, err)

		err = database.Commit(stateCommitment)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), edgePath, edgeNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leafPath, leafNode)
	})

	t.Run("Commit handles concurrent operations", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		numTries := 5
		tries := make([]struct {
			root         felt.Felt
			parent       felt.Felt
			classNodes   map[trieutils.Path]trienode.TrieNode
			stateComm    felt.Felt
			classRoot    felt.Felt
			contractRoot felt.Felt
		}, numTries)

		for i := range numTries {
			leafHash := *new(felt.Felt).SetUint64(uint64(i*100 + 50))
			rootHash := *new(felt.Felt).SetUint64(uint64(i * 100))

			leafPath := trieutils.NewBitArray(1, 0x00)
			leafNode := NewLeafWithHash([]byte{byte(i), byte(i + 1), byte(i + 2)}, leafHash)

			rootPath := trieutils.NewBitArray(0, 0x0)
			rootNode := trienode.NewNonLeaf(rootHash, createBinaryNodeBlob(leafHash, felt.Zero))

			tries[i] = struct {
				root         felt.Felt
				parent       felt.Felt
				classNodes   map[trieutils.Path]trienode.TrieNode
				stateComm    felt.Felt
				classRoot    felt.Felt
				contractRoot felt.Felt
			}{
				root:   rootHash,
				parent: *new(felt.Felt).SetUint64(uint64(i*100 - 1)),
				classNodes: map[trieutils.Path]trienode.TrieNode{
					rootPath: rootNode,
					leafPath: leafNode,
				},
				stateComm:    *new(felt.Felt).SetUint64(uint64(1000 + i)),
				classRoot:    rootHash,
				contractRoot: *new(felt.Felt).SetUint64(uint64(3000 + i)),
			}

			database.Update(tries[i].root, tries[i].parent, uint64(i), tries[i].classNodes, nil)

			err := mockStoreRootsForStateComm(memDB, tries[i].stateComm, tries[i].classRoot, tries[i].contractRoot)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		wg.Add(numTries)
		for i := range numTries {
			go func(i int) {
				defer wg.Done()
				err := database.Commit(tries[i].stateComm)
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

		leaf1Hash := *new(felt.Felt).SetUint64(201)
		leaf2Hash := *new(felt.Felt).SetUint64(202)
		rootHash := *new(felt.Felt).SetUint64(100)

		leaf1Path := trieutils.NewBitArray(1, 0x00)
		leaf1Node := NewLeafWithHash([]byte{1, 2, 3}, leaf1Hash)

		leaf2Path := trieutils.NewBitArray(1, 0x01)
		leaf2Node := NewLeafWithHash([]byte{4, 5, 6}, leaf2Hash)

		rootPath := trieutils.NewBitArray(0, 0x0)
		rootNode := trienode.NewNonLeaf(rootHash, createBinaryNodeBlob(leaf1Hash, leaf2Hash))

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:  rootNode,
			leaf1Path: leaf1Node,
			leaf2Path: leaf2Node,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(felt.Zero, felt.Zero, 42, classNodes, contractNodes)

		classNodes = map[trieutils.Path]trienode.TrieNode{
			leaf1Path: trienode.NewDeleted(true),
		}

		database.Update(felt.Zero, felt.Zero, 42, classNodes, contractNodes)

		classRoot := rootHash
		contractRoot := *new(felt.Felt).SetUint64(132)
		stateCommitment := *new(felt.Felt).SetUint64(133)

		err := mockStoreRootsForStateComm(memDB, stateCommitment, classRoot, contractRoot)
		require.NoError(t, err)

		err = database.Commit(stateCommitment)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)

		reader, err := database.NodeReader(trieutils.NewClassTrieID(felt.Zero))
		require.NoError(t, err)
		_, err = reader.Node(felt.Zero, leaf1Path, leaf1Node.Hash(), leaf1Node.IsLeaf())
		require.Error(t, err)
	})

	t.Run("getRootsForStateHash returns correct roots", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		classRoot := *new(felt.Felt).SetUint64(201)
		contractRoot := *new(felt.Felt).SetUint64(202)
		stateCommitment := *new(felt.Felt).SetUint64(203)

		err := mockStoreRootsForStateComm(memDB, stateCommitment, classRoot, contractRoot)
		require.NoError(t, err)

		gotClassRoot, gotContractRoot, err := database.getRootsForStateCommitment(stateCommitment)
		require.NoError(t, err)

		assert.Equal(t, classRoot, *gotClassRoot)
		assert.Equal(t, contractRoot, *gotContractRoot)

		modifiedClassRoot := *new(felt.Felt).SetUint64(301)
		modifiedContractRoot := *new(felt.Felt).SetUint64(302)

		err = mockStoreRootsForStateComm(memDB, stateCommitment, modifiedClassRoot, modifiedContractRoot)
		require.NoError(t, err)

		gotClassRoot, gotContractRoot, err = database.getRootsForStateCommitment(stateCommitment)
		require.NoError(t, err)

		assert.Equal(t, classRoot, *gotClassRoot)
		assert.Equal(t, contractRoot, *gotContractRoot)
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

		leaf1Hash := *new(felt.Felt).SetUint64(201)
		leaf2Hash := *new(felt.Felt).SetUint64(202)
		rootHash := *new(felt.Felt).SetUint64(100)

		leaf1Path := trieutils.NewBitArray(1, 0x00)
		leaf1Node := NewLeafWithHash([]byte{1, 2, 3}, leaf1Hash)

		leaf2Path := trieutils.NewBitArray(1, 0x01)
		leaf2Node := NewLeafWithHash([]byte{4, 5, 6}, leaf2Hash)

		rootPath := trieutils.NewBitArray(0, 0x0)
		rootNode := trienode.NewNonLeaf(rootHash, createBinaryNodeBlob(leaf1Hash, leaf2Hash))

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:  rootNode,
			leaf1Path: leaf1Node,
			leaf2Path: leaf2Node,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(felt.Zero, felt.Zero, 42, classNodes, contractNodes)

		initialSize := database.dirtyCacheSize
		err := database.Cap(uint64(initialSize / 2))
		require.NoError(t, err)

		assert.Less(t, database.dirtyCacheSize, initialSize)

		classRoot := rootHash
		contractRoot := *new(felt.Felt).SetUint64(402)
		stateCommitment := *new(felt.Felt).SetUint64(403)

		err = mockStoreRootsForStateComm(memDB, stateCommitment, classRoot, contractRoot)
		require.NoError(t, err)

		err = database.Commit(stateCommitment)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf1Path, leaf1Node)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)
	})

	t.Run("Cap to size larger than cache", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		rootHash := *new(felt.Felt).SetUint64(100)
		rootPath := trieutils.NewBitArray(0, 0x0)
		rootNode := trienode.NewNonLeaf(rootHash, createEdgeNodeBlob(felt.Zero))

		classNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath: rootNode,
		}

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{}

		database.Update(felt.Zero, felt.Zero, 42, classNodes, contractNodes)

		initialSize := database.dirtyCacheSize
		err := database.Cap(uint64(initialSize * 2))
		require.NoError(t, err)

		assert.Equal(t, initialSize, database.dirtyCacheSize)

		classRoot := rootHash
		contractRoot := *new(felt.Felt).SetUint64(502)
		stateCommitment := *new(felt.Felt).SetUint64(503)

		err = mockStoreRootsForStateComm(memDB, stateCommitment, classRoot, contractRoot)
		require.NoError(t, err)

		err = database.Commit(stateCommitment)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
	})
}
