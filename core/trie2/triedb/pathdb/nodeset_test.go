package pathdb

import (
	"bytes"
	"math"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	path1  = *new(trieutils.Path).SetUint64(10, 0x01)
	path2  = *new(trieutils.Path).SetUint64(10, 0x02)
	path3  = *new(trieutils.Path).SetUint64(10, 0x03)
	path5  = *new(trieutils.Path).SetUint64(10, 0x05)
	pathff = *new(trieutils.Path).SetUint64(10, 0xff)
	feltff = *felt.NewFromBytes[felt.Address]([]byte{0xff})
)

func TestNewNodeSet(t *testing.T) {
	classNodes := make(classNodesMap)
	contractNodes := make(contractNodesMap)
	contractStorage := make(contractStorageNodesMap)

	classNodes[path1] = trienode.NewLeaf(felt.Zero, []byte{0xaa})
	contractNodes[path2] = trienode.NewLeaf(felt.Zero, []byte{0xbb})

	owner := felt.Address{}
	contractStorage[owner] = make(map[trieutils.Path]trienode.TrieNode)
	contractStorage[owner][path3] = trienode.NewLeaf(felt.Zero, []byte{0xcc})

	ns := newNodeSet(classNodes, contractNodes, contractStorage)

	assert.Equal(t, 1, len(ns.classNodes))
	assert.Equal(t, 1, len(ns.contractNodes))
	assert.Equal(t, 1, len(ns.contractStorageNodes))
}

func TestGetNode(t *testing.T) {
	class := []byte("class")
	contract := []byte("contract")
	storage := []byte("storage")
	owner := felt.FromBytes[felt.Address]([]byte("owner"))

	ns := newNodeSet(
		map[trieutils.Path]trienode.TrieNode{
			path1: trienode.NewLeaf(felt.Zero, class),
		},
		map[trieutils.Path]trienode.TrieNode{
			path2: trienode.NewLeaf(felt.Zero, contract),
		},
		map[felt.Address]map[trieutils.Path]trienode.TrieNode{
			owner: {
				path3: trienode.NewLeaf(felt.Zero, storage),
			},
		},
	)

	t.Run("class node", func(t *testing.T) {
		node, ok := ns.node(&felt.Address{}, &path1, true)
		require.True(t, ok)
		require.Equal(t, class, node.Blob())
	})

	t.Run("contract node", func(t *testing.T) {
		node, ok := ns.node(&felt.Address{}, &path2, false)
		require.True(t, ok)
		require.Equal(t, contract, node.Blob())
	})

	t.Run("storage node", func(t *testing.T) {
		node, ok := ns.node(&owner, &path3, false)
		require.True(t, ok)
		require.Equal(t, storage, node.Blob())
	})

	t.Run("non-existent node", func(t *testing.T) {
		_, ok := ns.node(&felt.Address{}, &pathff, true)
		require.False(t, ok)
	})
}

func TestMerge(t *testing.T) {
	old := "old"
	added := "added"
	updated := "updated"

	baseSet := newNodeSet(
		map[trieutils.Path]trienode.TrieNode{
			path1: trienode.NewLeaf(felt.Zero, []byte(old)),
		},
		map[trieutils.Path]trienode.TrieNode{
			path2: trienode.NewLeaf(felt.Zero, []byte(old)),
		},
		map[felt.Address]map[trieutils.Path]trienode.TrieNode{
			feltff: {
				path3: trienode.NewLeaf(felt.Zero, []byte(old)),
			},
		},
	)

	mergeSet := newNodeSet(
		map[trieutils.Path]trienode.TrieNode{
			path1:  trienode.NewLeaf(felt.Zero, []byte(updated)),
			pathff: trienode.NewLeaf(felt.Zero, []byte(added)),
		},
		map[trieutils.Path]trienode.TrieNode{
			path2: trienode.NewLeaf(felt.Zero, []byte(updated)),
		},
		map[felt.Address]map[trieutils.Path]trienode.TrieNode{
			feltff: {
				path3: trienode.NewLeaf(felt.Zero, []byte(updated)),
				path5: trienode.NewLeaf(felt.Zero, []byte(added)),
			},
		},
	)

	baseSet.merge(mergeSet)
	assert.Equal(t, 2, len(baseSet.classNodes))
	assert.Equal(t, []byte(updated), baseSet.classNodes[path1].Blob())
	assert.Equal(t, []byte(added), baseSet.classNodes[pathff].Blob())

	assert.Equal(t, 1, len(baseSet.contractNodes))
	assert.Equal(t, []byte(updated), baseSet.contractNodes[path2].Blob())

	assert.Equal(t, 1, len(baseSet.contractStorageNodes))
	assert.Equal(t, 2, len(baseSet.contractStorageNodes[feltff]))
	assert.Equal(t, []byte(updated), baseSet.contractStorageNodes[feltff][path3].Blob())
	assert.Equal(t, []byte(added), baseSet.contractStorageNodes[feltff][path5].Blob())

	expectedSize := uint64(
		(len("updated") + trieutils.PathSize) + (len("added") + trieutils.PathSize) +
			(len("updated") + trieutils.PathSize) +
			ownerSize + (len("updated") + trieutils.PathSize) + ownerSize + (len("added") + trieutils.PathSize),
	)
	require.Equal(t, expectedSize, baseSet.size)
}

func TestEdgeCases(t *testing.T) {
	t.Run("size overflow", func(t *testing.T) {
		ns := &nodeSet{size: math.MaxUint64}
		ns.updateSize(1)
		require.Equal(t, uint64(math.MaxUint64), ns.size)
	})

	t.Run("size reset", func(t *testing.T) {
		ns := newNodeSet(nil, nil, nil)
		ns.reset()
		require.Equal(t, uint64(0), ns.size)
	})
}

func TestEnc(t *testing.T) {
	// Create test data
	classPath := *new(trieutils.Path).SetUint64(10, 0x01)
	contractPath := *new(trieutils.Path).SetUint64(10, 0x02)
	storagePath := *new(trieutils.Path).SetUint64(10, 0x03)

	owner1 := felt.FromBytes[felt.Address]([]byte{0x01})
	owner2 := felt.FromBytes[felt.Address]([]byte{0x02})
	classNode := trienode.NewLeaf(felt.Zero, []byte("class data"))
	contractNode := trienode.NewLeaf(felt.Zero, []byte("contract data"))
	storageNode1 := trienode.NewLeaf(felt.Zero, []byte("storage data 1"))
	storageNode2 := trienode.NewLeaf(felt.Zero, []byte("storage data 2"))
	// Create original nodeSet
	original := &nodeSet{
		classNodes: classNodesMap{
			classPath: classNode,
		},
		contractNodes: contractNodesMap{
			contractPath: contractNode,
		},
		contractStorageNodes: contractStorageNodesMap{
			owner1: {
				storagePath: storageNode1,
			},
			owner2: {
				storagePath: storageNode2,
			},
		},
	}

	// Encode
	var buf bytes.Buffer
	err := original.encode(&buf)
	require.NoError(t, err)

	// Decode into new nodeSet
	decoded := &nodeSet{}
	err = decoded.decode(buf.Bytes())
	require.NoError(t, err)

	// Verify class nodes
	assert.Equal(t, len(original.classNodes), len(decoded.classNodes))
	decodedClassNode, exists := decoded.classNodes[classPath]
	require.True(t, exists)
	assert.Equal(t, classNode.Blob(), decodedClassNode.Blob())

	// Verify contract nodes
	assert.Equal(t, len(original.contractNodes), len(decoded.contractNodes))
	decodedContractNode, exists := decoded.contractNodes[contractPath]
	require.True(t, exists)
	assert.Equal(t, contractNode.Blob(), decodedContractNode.Blob())

	// Verify contract storage nodes
	assert.Equal(t, len(original.contractStorageNodes), len(decoded.contractStorageNodes))
	decodedStorageNodes, exists := decoded.contractStorageNodes[owner1]
	require.True(t, exists)
	decodedStorageNode, exists := decodedStorageNodes[storagePath]
	require.True(t, exists)
	assert.Equal(t, storageNode1.Blob(), decodedStorageNode.Blob())

	decodedStorageNodes, exists = decoded.contractStorageNodes[owner2]
	require.True(t, exists)
	decodedStorageNode, exists = decodedStorageNodes[storagePath]
	require.True(t, exists)
	assert.Equal(t, storageNode2.Blob(), decodedStorageNode.Blob())
}

func TestEncEmpty(t *testing.T) {
	original := &nodeSet{
		classNodes:           make(classNodesMap),
		contractNodes:        make(contractNodesMap),
		contractStorageNodes: make(contractStorageNodesMap),
	}

	var buf bytes.Buffer
	err := original.encode(&buf)
	require.NoError(t, err)

	decoded := &nodeSet{}
	err = decoded.decode(buf.Bytes())
	require.NoError(t, err)

	assert.Empty(t, decoded.classNodes)
	assert.Empty(t, decoded.contractNodes)
	assert.Empty(t, decoded.contractStorageNodes)
}
