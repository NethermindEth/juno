package rawdb

import (
	"maps"
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

var (
	leaf1Hash = felt.NewFromUint64[felt.Felt](201)
	leaf2Hash = felt.NewFromUint64[felt.Felt](202)
	rootHash  = felt.NewFromUint64[felt.Felt](100)

	leaf1Path = trieutils.NewBitArray(1, 0x00)
	leaf2Path = trieutils.NewBitArray(1, 0x01)
	rootPath  = trieutils.NewBitArray(0, 0x0)

	leaf1Node = trienode.NewLeaf(*leaf1Hash, []byte{1, 2, 3})
	leaf2Node = trienode.NewLeaf(*leaf2Hash, []byte{4, 5, 6})
	rootNode  = trienode.NewNonLeaf(*rootHash, createBinaryNodeBlob(leaf1Hash, leaf2Hash))

	basicClassNodes = map[trieutils.Path]trienode.TrieNode{
		rootPath:  rootNode,
		leaf1Path: leaf1Node,
		leaf2Path: leaf2Node,
	}
)

func createBinaryNodeBlob(leftHash, rightHash *felt.Felt) []byte {
	binaryBlob := make([]byte, 1+2*felt.Bytes)
	binaryBlob[0] = 1

	leftBytes := leftHash.Bytes()
	rightBytes := rightHash.Bytes()

	copy(binaryBlob[1:felt.Bytes+1], leftBytes[:])
	copy(binaryBlob[felt.Bytes+1:], rightBytes[:])

	return binaryBlob
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
		if felt.IsZero(&owner) {
			for path, node := range ownerNodes {
				ownerSet.Add(&path, node)
			}
		} else {
			childSet := trienode.NewNodeSet(owner)
			for path, node := range ownerNodes {
				childSet.Add(&path, node)
			}
			childSets[owner] = &childSet
		}
	}

	return &trienode.MergeNodeSet{
		OwnerSet:  &ownerSet,
		ChildSets: childSets,
	}
}

func verifyNode(
	t *testing.T,
	database *Database,
	id trieutils.TrieID,
	path *trieutils.Path,
	node trienode.TrieNode,
) {
	t.Helper()

	reader, err := database.NodeReader(id)
	require.NoError(t, err)

	owner := id.Owner()
	nodeHash := node.Hash()
	blob, err := reader.Node(
		&owner,
		path,
		(*felt.Hash)(&nodeHash),
		node.IsLeaf(),
	)
	require.NoError(t, err)
	assert.Equal(t, node.Blob(), blob)
}

func TestRawDB(t *testing.T) {
	t.Run("New creates database", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB)
		require.NotNil(t, database)
	})

	t.Run("Update with all node types", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB)

		contractHash := felt.NewFromUint64[felt.Felt](210)
		contractPath := trieutils.NewBitArray(1, 0x01)
		contractNode := trienode.NewLeaf(*contractHash, []byte{7, 8, 9})

		contractOwner := felt.NewFromUint64[felt.Address](123)
		storageHash := felt.NewFromUint64[felt.Felt](220)
		storagePath := trieutils.NewBitArray(1, 0x02)
		storageNode := trienode.NewLeaf(*storageHash, []byte{10, 11, 12})

		contractNodes := map[felt.Address]map[trieutils.Path]trienode.TrieNode{
			{}: {
				contractPath: contractNode,
			},
		}

		contractStorageNodes := map[felt.Address]map[trieutils.Path]trienode.TrieNode{
			*contractOwner: {
				storagePath: storageNode,
			},
		}

		allContractNodes := make(map[felt.Address]map[trieutils.Path]trienode.TrieNode)
		maps.Copy(allContractNodes, contractNodes)
		for owner, nodes := range contractStorageNodes {
			if _, exists := allContractNodes[owner]; !exists {
				allContractNodes[owner] = make(map[trieutils.Path]trienode.TrieNode)
			}
			for path, node := range nodes {
				allContractNodes[owner][path] = node
			}
		}

		batch := memDB.NewBatch()
		err := database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			1,
			createMergeNodeSet(basicClassNodes),
			createContractMergeNodeSet(allContractNodes),
			batch,
		)
		require.NoError(t, err)
		require.NoError(t, batch.Write())

		classID := trieutils.NewClassTrieID(felt.StateRootHash{})
		verifyNode(t, database, classID, &rootPath, rootNode)
		verifyNode(t, database, classID, &leaf1Path, leaf1Node)
		verifyNode(t, database, classID, &leaf2Path, leaf2Node)

		contractID := trieutils.NewContractTrieID(felt.StateRootHash{})
		verifyNode(t, database, contractID, &contractPath, contractNode)

		storageID := trieutils.NewContractStorageTrieID(felt.StateRootHash{}, *contractOwner)
		verifyNode(t, database, storageID, &storagePath, storageNode)
	})

	t.Run("Update with deleted nodes", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB)

		batch := memDB.NewBatch()
		err := database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			1,
			createMergeNodeSet(basicClassNodes),
			nil,
			batch,
		)
		require.NoError(t, err)
		require.NoError(t, batch.Write())

		classID := trieutils.NewClassTrieID(felt.StateRootHash{})
		verifyNode(t, database, classID, &leaf1Path, leaf1Node)

		deletedNodes := map[trieutils.Path]trienode.TrieNode{
			leaf1Path: trienode.NewDeleted(true),
		}

		batch = memDB.NewBatch()
		err = database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			2,
			createMergeNodeSet(deletedNodes),
			nil,
			batch,
		)
		require.NoError(t, err)
		require.NoError(t, batch.Write())

		reader, err := database.NodeReader(classID)
		require.NoError(t, err)

		owner := felt.Address{}
		leaf1Hash := leaf1Node.Hash()
		_, err = reader.Node(
			&owner,
			&leaf1Path,
			(*felt.Hash)(&leaf1Hash),
			true,
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, db.ErrKeyNotFound)
	})

	t.Run("NodeReader returns correct reader", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB)

		batch := memDB.NewBatch()
		err := database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			1,
			createMergeNodeSet(basicClassNodes),
			nil,
			batch,
		)
		require.NoError(t, err)
		require.NoError(t, batch.Write())

		classID := trieutils.NewClassTrieID(felt.StateRootHash{})
		reader, err := database.NodeReader(classID)
		require.NoError(t, err)
		require.NotNil(t, reader)

		owner := felt.Address{}
		rootHash := rootNode.Hash()
		blob, err := reader.Node(
			&owner,
			&rootPath,
			(*felt.Hash)(&rootHash),
			false,
		)
		require.NoError(t, err)
		assert.Equal(t, rootNode.Blob(), blob)
	})

	t.Run("Multiple updates", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB)

		batch := memDB.NewBatch()
		err := database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			1,
			createMergeNodeSet(basicClassNodes),
			nil,
			batch,
		)
		require.NoError(t, err)
		require.NoError(t, batch.Write())

		classID := trieutils.NewClassTrieID(felt.StateRootHash{})
		verifyNode(t, database, classID, &rootPath, rootNode)
		verifyNode(t, database, classID, &leaf1Path, leaf1Node)
		verifyNode(t, database, classID, &leaf2Path, leaf2Node)

		newLeafHash := felt.NewFromUint64[felt.Felt](203)
		newLeafPath := trieutils.NewBitArray(2, 0x02)
		newLeafNode := trienode.NewLeaf(*newLeafHash, []byte{13, 14, 15})

		newNodes := map[trieutils.Path]trienode.TrieNode{
			newLeafPath: newLeafNode,
		}

		batch = memDB.NewBatch()
		err = database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			2,
			createMergeNodeSet(newNodes),
			nil,
			batch,
		)
		require.NoError(t, err)
		require.NoError(t, batch.Write())

		verifyNode(t, database, classID, &newLeafPath, newLeafNode)
		verifyNode(t, database, classID, &rootPath, rootNode)
		verifyNode(t, database, classID, &leaf1Path, leaf1Node)
		verifyNode(t, database, classID, &leaf2Path, leaf2Node)
	})

	t.Run("Concurrent reads", func(t *testing.T) {
		memDB := memory.New()
		database := New(memDB)

		batch := memDB.NewBatch()
		err := database.Update(
			&felt.StateRootHash{},
			&felt.StateRootHash{},
			1,
			createMergeNodeSet(basicClassNodes),
			nil,
			batch,
		)
		require.NoError(t, err)
		require.NoError(t, batch.Write())

		classID := trieutils.NewClassTrieID(felt.StateRootHash{})
		owner := felt.Address{}

		const numGoroutines = 20
		const readsPerGoroutine = 10
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for range numGoroutines {
			go func() {
				defer wg.Done()
				reader, err := database.NodeReader(classID)
				if err != nil {
					return
				}

				for range readsPerGoroutine {
					rootHash := rootNode.Hash()
					_, _ = reader.Node(&owner, &rootPath, (*felt.Hash)(&rootHash), false)
				}
			}()
		}

		wg.Wait()
	})
}
