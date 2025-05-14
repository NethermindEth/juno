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

var (
	leaf1Hash   = *new(felt.Felt).SetUint64(201)
	leaf2Hash   = *new(felt.Felt).SetUint64(202)
	rootHash    = *new(felt.Felt).SetUint64(100)
	level1Hash1 = *new(felt.Felt).SetUint64(201)
	level1Hash2 = *new(felt.Felt).SetUint64(202)

	leaf1Path   = trieutils.NewBitArray(1, 0x00)
	leaf2Path   = trieutils.NewBitArray(1, 0x01)
	rootPath    = trieutils.NewBitArray(0, 0x0)
	level1Path1 = trieutils.NewBitArray(1, 0x00)
	level1Path2 = trieutils.NewBitArray(1, 0x01)

	leaf1Node   = NewLeafWithHash([]byte{1, 2, 3}, leaf1Hash)
	leaf2Node   = NewLeafWithHash([]byte{4, 5, 6}, leaf2Hash)
	rootNode    = trienode.NewNonLeaf(rootHash, createBinaryNodeBlob(leaf1Hash, leaf2Hash))
	level1Node1 = trienode.NewNonLeaf(level1Hash1, createEdgeNodeBlob(leaf1Hash))
	level1Node2 = trienode.NewNonLeaf(level1Hash2, createEdgeNodeBlob(leaf2Hash))

	basicClassNodes = map[trieutils.Path]trienode.TrieNode{
		rootPath:  rootNode,
		leaf1Path: leaf1Node,
		leaf2Path: leaf2Node,
	}
)

func verifyNode(t *testing.T, database *Database, id trieutils.TrieID, path trieutils.Path, node trienode.TrieNode) {
	t.Helper()

	reader, err := database.NodeReader(id)
	require.NoError(t, err)

	blob, err := reader.Node(id.Owner(), path, node.Hash(), node.IsLeaf())
	require.NoError(t, err)
	assert.Equal(t, node.Blob(), blob)

	key := trieutils.NodeKeyByHash(id.Bucket(), id.Owner(), path, node.Hash(), node.IsLeaf())
	_, found := database.dirtyCache.Get(key, bucketToTrieType(id.Bucket()), id.Owner())
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

func createMergeNodeSet(nodes map[trieutils.Path]trienode.TrieNode) *trienode.MergeNodeSet {
	ownerSet := trienode.NewNodeSet(felt.Zero)
	for path, node := range nodes {
		ownerSet.Add(path, node)
	}
	return trienode.NewMergeNodeSet(ownerSet)
}

func createContractMergeNodeSet(nodes map[felt.Felt]map[trieutils.Path]trienode.TrieNode) *trienode.MergeNodeSet {
	ownerSet := trienode.NewNodeSet(felt.Zero)
	childSets := make(map[felt.Felt]*trienode.NodeSet)

	for owner, ownerNodes := range nodes {
		childSet := trienode.NewNodeSet(owner)
		for path, node := range ownerNodes {
			childSet.Add(path, node)
		}
		childSets[owner] = childSet
	}

	return &trienode.MergeNodeSet{
		OwnerSet:  ownerSet,
		ChildSets: childSets,
	}
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
		}
		database := New(memDB, config)
		assert.NotNil(t, database)
	})

	t.Run("Update and Commit basic trie structure", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, DefaultConfig)

		err := database.Update(felt.Zero, felt.Zero, 42, createMergeNodeSet(basicClassNodes), createContractMergeNodeSet(nil))
		require.NoError(t, err)

		err = database.Commit(felt.Zero)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf1Path, leaf1Node)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)
	})

	t.Run("Update and Commit deep trie structure", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, DefaultConfig)

		deepClassNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:    rootNode,
			level1Path1: level1Node1,
			level1Path2: level1Node2,
			leaf1Path:   leaf1Node,
			leaf2Path:   leaf2Node,
		}

		err := database.Update(felt.Zero, felt.Zero, 42, createMergeNodeSet(deepClassNodes), createContractMergeNodeSet(nil))
		require.NoError(t, err)

		err = database.Commit(felt.Zero)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), level1Path1, level1Node1)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), level1Path2, level1Node2)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf1Path, leaf1Node)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)
	})

	t.Run("Update and Commit with contract nodes", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, DefaultConfig)

		contractHash := *new(felt.Felt).SetUint64(210)
		contractOwner := *new(felt.Felt).SetUint64(123)
		contractPath := trieutils.NewBitArray(1, 0x01)
		contractNode := NewLeafWithHash([]byte{4, 5, 6}, contractHash)

		contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{
			contractOwner: {
				contractPath: contractNode,
			},
		}

		err := database.Update(felt.Zero, felt.Zero, 42, createMergeNodeSet(basicClassNodes), createContractMergeNodeSet(contractNodes))
		require.NoError(t, err)

		err = database.Commit(felt.Zero)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewContractStorageTrieID(felt.Zero, contractOwner), contractPath, contractNode)
	})

	t.Run("Update and Commit deep trie structure with edge nodes", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, DefaultConfig)

		edgeHash := *new(felt.Felt).SetUint64(201)
		edgePath := trieutils.NewBitArray(1, 0x01)
		edgeNode := trienode.NewNonLeaf(edgeHash, createEdgeNodeBlob(leaf1Hash))

		edgeClassNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:  rootNode,
			edgePath:  edgeNode,
			leaf1Path: leaf1Node,
		}

		err := database.Update(felt.Zero, felt.Zero, 42, createMergeNodeSet(edgeClassNodes), createContractMergeNodeSet(nil))
		require.NoError(t, err)

		err = database.Commit(felt.Zero)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), edgePath, edgeNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf1Path, leaf1Node)
	})

	t.Run("Commit handles concurrent operations", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, DefaultConfig)

		numTries := 5
		tries := make([]struct {
			root         felt.Felt
			parent       felt.Felt
			classNodes   map[trieutils.Path]trienode.TrieNode
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
				classRoot    felt.Felt
				contractRoot felt.Felt
			}{
				root:   rootHash,
				parent: *new(felt.Felt).SetUint64(uint64(i*100 - 1)),
				classNodes: map[trieutils.Path]trienode.TrieNode{
					rootPath: rootNode,
					leafPath: leafNode,
				},
				classRoot:    rootHash,
				contractRoot: *new(felt.Felt).SetUint64(uint64(3000 + i)),
			}

			err := database.Update(tries[i].root, tries[i].parent, uint64(i), createMergeNodeSet(tries[i].classNodes), createContractMergeNodeSet(nil))
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		wg.Add(numTries)
		for range numTries {
			go func() {
				defer wg.Done()
				err := database.Commit(felt.Zero)
				require.NoError(t, err)
			}()
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
		database := New(memDB, DefaultConfig)

		err := database.Update(felt.Zero, felt.Zero, 42, createMergeNodeSet(basicClassNodes), createContractMergeNodeSet(nil))
		require.NoError(t, err)

		deletedNodes := map[trieutils.Path]trienode.TrieNode{
			leaf1Path: trienode.NewDeleted(true),
		}

		err = database.Update(felt.Zero, felt.Zero, 42, createMergeNodeSet(deletedNodes), createContractMergeNodeSet(nil))
		require.NoError(t, err)

		err = database.Commit(felt.Zero)
		require.NoError(t, err)

		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), rootPath, rootNode)
		verifyNode(t, database, trieutils.NewClassTrieID(felt.Zero), leaf2Path, leaf2Node)

		reader, err := database.NodeReader(trieutils.NewClassTrieID(felt.Zero))
		require.NoError(t, err)
		_, err = reader.Node(felt.Zero, leaf1Path, leaf1Node.Hash(), leaf1Node.IsLeaf())
		require.Error(t, err)
	})
}
