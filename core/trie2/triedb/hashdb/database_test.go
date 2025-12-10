package hashdb

import (
	"math"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

var (
	leaf1Hash   = felt.NewFromUint64[felt.Felt](201)
	leaf2Hash   = felt.NewFromUint64[felt.Felt](202)
	rootHash    = felt.NewFromUint64[felt.Felt](100)
	level1Hash1 = felt.NewFromUint64[felt.Felt](301)
	level1Hash2 = felt.NewFromUint64[felt.Felt](302)

	leaf1Path   = trieutils.NewBitArray(1, 0x00)
	leaf2Path   = trieutils.NewBitArray(1, 0x01)
	rootPath    = trieutils.NewBitArray(0, 0x0)
	level1Path1 = trieutils.NewBitArray(2, 0x00)
	level1Path2 = trieutils.NewBitArray(2, 0x01)

	leaf1Node   = trienode.NewLeaf(*leaf1Hash, []byte{1, 2, 3})
	leaf2Node   = trienode.NewLeaf(*leaf2Hash, []byte{4, 5, 6})
	rootNode    = trienode.NewNonLeaf(*rootHash, createBinaryNodeBlob(leaf1Hash, leaf2Hash))
	level1Node1 = trienode.NewNonLeaf(*level1Hash1, createEdgeNodeBlob(leaf1Hash))
	level1Node2 = trienode.NewNonLeaf(*level1Hash2, createEdgeNodeBlob(leaf2Hash))

	basicClassNodes = map[trieutils.Path]trienode.TrieNode{
		rootPath:  rootNode,
		leaf1Path: leaf1Node,
		leaf2Path: leaf2Node,
	}
)

// verifyNode verifies that the node is stored in the database and that the database returns the correct node.
// It also checks that the node is not in the dirty cache, which mean that it has been flushed to disk.
func verifyNodeInDisk(t *testing.T, database *Database, id trieutils.TrieID, path *trieutils.Path, node trienode.TrieNode) {
	t.Helper()

	reader, err := database.NodeReader(id)
	require.NoError(t, err)

	owner := id.Owner()
	nodeHash := node.Hash()
	// TODO(maksym): update to make nodeHash a felt.Hash instead of felt.Felt
	_, found := database.dirtyCache.getNode(
		&owner,
		path,
		(*felt.Hash)(&nodeHash),
		id.Bucket() == db.ClassTrie,
	)
	assert.False(t, found)
	blob, err := reader.Node(&owner, path, (*felt.Hash)(&nodeHash), node.IsLeaf())
	require.NoError(t, err)
	assert.Equal(t, node.Blob(), blob)
}

func verifyNodeInDirtyCache(t *testing.T, database *Database, id trieutils.TrieID, path *trieutils.Path, node trienode.TrieNode) {
	t.Helper()

	owner := id.Owner()
	nodeHash := node.Hash()
	// TODO(maksym): update to make nodeHash a felt.Hash instead of felt.Felt
	_, found := database.dirtyCache.getNode(
		&owner,
		path,
		(*felt.Hash)(&nodeHash),
		id.Bucket() == db.ClassTrie,
	)
	assert.True(t, found)
}

func createBinaryNodeBlob(leftHash, rightHash *felt.Felt) []byte {
	binaryBlob := make([]byte, 1+2*felt.Bytes)
	binaryBlob[0] = 1

	leftBytes := leftHash.Bytes()
	rightBytes := rightHash.Bytes()

	copy(binaryBlob[1:felt.Bytes+1], leftBytes[:])
	copy(binaryBlob[felt.Bytes+1:], rightBytes[:])

	return binaryBlob
}

func createEdgeNodeBlob(childHash *felt.Felt) []byte {
	edgeBlob := make([]byte, 1+felt.Bytes)
	edgeBlob[0] = 2

	childBytes := childHash.Bytes()
	copy(edgeBlob[1:felt.Bytes+1], childBytes[:])

	return edgeBlob
}

func createMergeNodeSet(nodes map[trieutils.Path]trienode.TrieNode) *trienode.MergeNodeSet {
	ownerSet := trienode.NewNodeSet(felt.Address{})
	for path, node := range nodes {
		ownerSet.Add(&path, node)
	}
	return trienode.NewMergeNodeSet(&ownerSet)
}

func createContractMergeNodeSet(
	nodes map[felt.Address]map[trieutils.Path]trienode.TrieNode,
) *trienode.MergeNodeSet {
	ownerSet := trienode.NewNodeSet(felt.Address{})
	childSets := make(map[felt.Address]*trienode.NodeSet)

	for owner, ownerNodes := range nodes {
		childSet := trienode.NewNodeSet(owner)
		for path, node := range ownerNodes {
			childSet.Add(&path, node)
		}
		childSets[owner] = &childSet
	}

	return &trienode.MergeNodeSet{
		OwnerSet:  &ownerSet,
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
			CleanCacheSize: 1024,
		}
		database := New(memDB, config)
		assert.NotNil(t, database)
	})

	t.Run("panics when cache size is too large but not max uint64", func(t *testing.T) {
		assert.PanicsWithValue(t, "cache size too large: uint64 to int conversion would overflow", func() {
			newCleanCache(math.MaxInt64 + 1)
		})
	})

	t.Run("Update and Commit deep trie structure", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		deepClassNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:    rootNode,
			level1Path1: level1Node1,
			level1Path2: level1Node2,
			leaf1Path:   leaf1Node,
			leaf2Path:   leaf2Node,
		}

		err := database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			42,
			createMergeNodeSet(deepClassNodes),
			createContractMergeNodeSet(nil),
		)
		require.NoError(t, err)

		err = database.Commit(&felt.StateRootHash{})
		require.NoError(t, err)

		verifyNodeInDisk(t, database, trieutils.NewClassTrieID(felt.StateRootHash{}), &rootPath, rootNode)
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&level1Path1,
			level1Node1,
		)
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&level1Path2,
			level1Node2,
		)
		verifyNodeInDisk(
			t, database, trieutils.NewClassTrieID(felt.StateRootHash{}), &leaf1Path, leaf1Node,
		)
		verifyNodeInDisk(
			t, database, trieutils.NewClassTrieID(felt.StateRootHash{}), &leaf2Path, leaf2Node,
		)
	})

	t.Run("Update and Commit with contract nodes and storage", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		contractHash := felt.FromUint64[felt.Felt](210)
		contractOwner := felt.FromUint64[felt.Address](123)
		contractPath := trieutils.NewBitArray(1, 0x01)
		contractNode := trienode.NewLeaf(contractHash, []byte{4, 5, 6})

		storageHash := felt.FromUint64[felt.Felt](220)
		storagePath := trieutils.NewBitArray(1, 0x02)
		storageNode := trienode.NewLeaf(storageHash, []byte{7, 8, 9})

		contractNodes := map[felt.Address]map[trieutils.Path]trienode.TrieNode{
			contractOwner: {
				contractPath: contractNode,
			},
		}

		contractStorageNodes := map[felt.Address]map[trieutils.Path]trienode.TrieNode{
			contractOwner: {
				storagePath: storageNode,
			},
		}

		allContractNodes := make(map[felt.Address]map[trieutils.Path]trienode.TrieNode)
		maps.Copy(allContractNodes, contractNodes)
		for owner, nodes := range contractStorageNodes {
			if _, exists := allContractNodes[owner]; !exists {
				allContractNodes[owner] = make(map[trieutils.Path]trienode.TrieNode)
			}
			maps.Copy(allContractNodes[owner], nodes)
		}

		err := database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			42,
			createMergeNodeSet(basicClassNodes),
			createContractMergeNodeSet(allContractNodes),
		)
		require.NoError(t, err)

		err = database.Commit(&felt.StateRootHash{})
		require.NoError(t, err)

		// Verify class nodes
		verifyNodeInDisk(t, database, trieutils.NewClassTrieID(felt.StateRootHash{}), &rootPath, rootNode)
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&leaf1Path,
			leaf1Node,
		)
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&leaf2Path,
			leaf2Node,
		)

		// Verify contract nodes
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewContractTrieID(felt.StateRootHash{}),
			&contractPath,
			contractNode,
		)

		// Verify contract storage nodes
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewContractStorageTrieID(felt.StateRootHash{}, contractOwner),
			&storagePath,
			storageNode,
		)
	})

	t.Run("Update and Commit deep trie structure with edge nodes", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		edgeHash := felt.FromUint64[felt.Felt](201)
		edgePath := trieutils.NewBitArray(1, 0x01)
		edgeNode := trienode.NewNonLeaf(edgeHash, createEdgeNodeBlob(leaf1Hash))

		edgeClassNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:  rootNode,
			edgePath:  edgeNode,
			leaf1Path: leaf1Node,
		}

		err := database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			42,
			createMergeNodeSet(edgeClassNodes),
			createContractMergeNodeSet(nil),
		)
		require.NoError(t, err)

		err = database.Commit(&felt.StateRootHash{})
		require.NoError(t, err)

		verifyNodeInDisk(t, database, trieutils.NewClassTrieID(felt.StateRootHash{}), &rootPath, rootNode)
		verifyNodeInDisk(t, database, trieutils.NewClassTrieID(felt.StateRootHash{}), &edgePath, edgeNode)
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&leaf1Path,
			leaf1Node,
		)
	})

	t.Run("Commit handles concurrent operations", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		numTries := 5
		tries := make([]struct {
			root         felt.StateRootHash
			parent       felt.StateRootHash
			classNodes   map[trieutils.Path]trienode.TrieNode
			classRoot    felt.Hash
			contractRoot felt.Hash
		}, numTries)

		for i := range numTries {
			leafHash := felt.NewFromUint64[felt.Hash](uint64(i*100 + 50))
			rootHash := felt.NewFromUint64[felt.StateRootHash](uint64(i * 100))

			leafPath := trieutils.NewBitArray(1, 0x00)
			leafNode := trienode.NewLeaf(felt.Felt(*leafHash), []byte{byte(i), byte(i + 1), byte(i + 2)})

			rootPath := trieutils.NewBitArray(0, 0x0)
			rootNode := trienode.NewNonLeaf(
				felt.Felt(*rootHash),
				createBinaryNodeBlob((*felt.Felt)(leafHash), &felt.Zero),
			)

			tries[i] = struct {
				root         felt.StateRootHash
				parent       felt.StateRootHash
				classNodes   map[trieutils.Path]trienode.TrieNode
				classRoot    felt.Hash
				contractRoot felt.Hash
			}{
				root:   *rootHash,
				parent: felt.FromUint64[felt.StateRootHash](uint64(i*100 - 1)),
				classNodes: map[trieutils.Path]trienode.TrieNode{
					rootPath: rootNode,
					leafPath: leafNode,
				},
				classRoot:    felt.Hash(*rootHash),
				contractRoot: felt.FromUint64[felt.Hash](uint64(3000 + i)),
			}

			err := database.Update(&tries[i].root, &tries[i].parent, uint64(i), createMergeNodeSet(tries[i].classNodes), createContractMergeNodeSet(nil))
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		wg.Add(numTries)
		for range numTries {
			go func() {
				defer wg.Done()
				err := database.Commit(&felt.StateRootHash{})
				require.NoError(t, err)
			}()
		}
		wg.Wait()

		for _, trie := range tries {
			for path, node := range trie.classNodes {
				verifyNodeInDisk(t, database, trieutils.NewClassTrieID(felt.StateRootHash{}), &path, node)
			}
		}
	})

	t.Run("Update and Commit with deleted nodes", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB, nil)

		err := database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			42,
			createMergeNodeSet(basicClassNodes),
			createContractMergeNodeSet(nil),
		)
		require.NoError(t, err)

		newRootHash := felt.FromUint64[felt.Felt](101)
		newRootNode := trienode.NewNonLeaf(newRootHash, createBinaryNodeBlob(&felt.Zero, leaf2Hash))

		updatedNodes := map[trieutils.Path]trienode.TrieNode{
			rootPath:  newRootNode,
			leaf2Path: leaf2Node,
			leaf1Path: trienode.NewDeleted(true),
		}

		err = database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			42,
			createMergeNodeSet(updatedNodes),
			createContractMergeNodeSet(nil),
		)
		require.NoError(t, err)

		verifyNodeInDirtyCache(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&rootPath,
			rootNode,
		)
		verifyNodeInDirtyCache(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&leaf1Path,
			leaf1Node,
		)
		verifyNodeInDirtyCache(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&leaf2Path,
			leaf2Node,
		)
		verifyNodeInDirtyCache(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&rootPath,
			newRootNode,
		)
		verifyNodeInDirtyCache(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&leaf2Path,
			leaf2Node,
		)

		err = database.Commit(&felt.StateRootHash{})
		require.NoError(t, err)

		verifyNodeInDisk(t, database, trieutils.NewClassTrieID(felt.StateRootHash{}), &rootPath, rootNode)
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&leaf1Path,
			leaf1Node,
		)
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&leaf2Path,
			leaf2Node,
		)
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&rootPath,
			newRootNode,
		)
		verifyNodeInDisk(
			t,
			database,
			trieutils.NewClassTrieID(felt.StateRootHash{}),
			&leaf2Path,
			leaf2Node,
		)
	})

	t.Run("GetTrieRootNodes", func(t *testing.T) {
		t.Run("successfully retrieves trie root nodes", func(t *testing.T) {
			memDB := memory.New()
			database := New(memDB, nil)

			stateCommitment := felt.NewFromUint64[felt.StateRootHash](1000)
			classRootBlob := createBinaryNodeBlob(leaf1Hash, leaf2Hash)
			contractRootBlob := createBinaryNodeBlob(leaf1Hash, leaf2Hash)
			classRootHash := crypto.Poseidon(leaf1Hash, leaf2Hash)
			contractRootHash := crypto.Poseidon(leaf1Hash, leaf2Hash)

			val := append(classRootHash.Marshal(), contractRootHash.Marshal()...)
			err := memDB.Put(db.StateHashToTrieRootsKey(stateCommitment), val)
			require.NoError(t, err)

			err = trieutils.WriteNodeByHash(
				memDB,
				db.ClassTrie,
				&felt.Address{},
				&rootPath,
				(*felt.Hash)(&classRootHash),
				false,
				classRootBlob,
			)
			require.NoError(t, err)
			err = trieutils.WriteNodeByHash(
				memDB,
				db.ContractTrieContract,
				&felt.Address{},
				&rootPath,
				(*felt.Hash)(&contractRootHash),
				false,
				contractRootBlob,
			)
			require.NoError(t, err)

			// TODO(maksym): update to make classRootHash and contractRootHash a felt.Hash instead of felt.Felt
			newClassRootNode, newContractRootNode, err := database.GetTrieRootNodes(
				(*felt.Hash)(&classRootHash),
				(*felt.Hash)(&contractRootHash),
			)
			require.NoError(t, err)
			assert.NotNil(t, newClassRootNode)
			assert.NotNil(t, newContractRootNode)
			newClassRootHash := newClassRootNode.Hash(crypto.Poseidon)
			newContractRootHash := newContractRootNode.Hash(crypto.Poseidon)

			assert.True(t, classRootHash.Equal(&newClassRootHash))
			assert.True(t, contractRootHash.Equal(&newContractRootHash))
		})

		t.Run("returns error when root nodes not found", func(t *testing.T) {
			memDB := memory.New()
			database := New(memDB, nil)

			stateCommitment := felt.NewFromUint64[felt.StateRootHash](1000)
			classRootHash := felt.NewFromUint64[felt.Hash](2000)
			contractRootHash := felt.NewFromUint64[felt.Hash](3000)

			val := append(classRootHash.Marshal(), contractRootHash.Marshal()...)
			err := memDB.Put(db.StateHashToTrieRootsKey(stateCommitment), val)
			require.NoError(t, err)

			_, _, err = database.GetTrieRootNodes(classRootHash, contractRootHash)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "class root node not found")
		})
	})
}
