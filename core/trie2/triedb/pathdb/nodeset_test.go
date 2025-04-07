package pathdb

import (
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
	feltff = *new(felt.Felt).SetBytes([]byte{0xff})
)

func TestNewNodeSet(t *testing.T) {
	classNodes := make(classNodesMap)
	contractNodes := make(contractNodesMap)
	contractStorage := make(contractStorageNodesMap)

	classNodes[path1] = trienode.NewLeaf([]byte{0xaa})
	contractNodes[path2] = trienode.NewLeaf([]byte{0xbb})

	owner := felt.Zero
	contractStorage[owner] = make(map[trieutils.Path]trienode.TrieNode)
	contractStorage[owner][path3] = trienode.NewLeaf([]byte{0xcc})

	ns := newNodeSet(classNodes, contractNodes, contractStorage)

	assert.Equal(t, 1, len(ns.classNodes))
	assert.Equal(t, 1, len(ns.contractNodes))
	assert.Equal(t, 1, len(ns.contractStorageNodes))
}

func TestGetNode(t *testing.T) {
	class := []byte("class")
	contract := []byte("contract")
	storage := []byte("storage")
	owner := *new(felt.Felt).SetBytes([]byte("owner"))

	ns := newNodeSet(
		map[trieutils.Path]trienode.TrieNode{
			path1: trienode.NewLeaf(class),
		},
		map[trieutils.Path]trienode.TrieNode{
			path2: trienode.NewLeaf(contract),
		},
		map[felt.Felt]map[trieutils.Path]trienode.TrieNode{
			owner: {
				path3: trienode.NewLeaf(storage),
			},
		},
	)

	t.Run("class node", func(t *testing.T) {
		node, ok := ns.node(felt.Zero, path1, true)
		require.True(t, ok)
		require.Equal(t, class, node.Blob())
	})

	t.Run("contract node", func(t *testing.T) {
		node, ok := ns.node(felt.Zero, path2, false)
		require.True(t, ok)
		require.Equal(t, contract, node.Blob())
	})

	t.Run("storage node", func(t *testing.T) {
		node, ok := ns.node(owner, path3, false)
		require.True(t, ok)
		require.Equal(t, storage, node.Blob())
	})

	t.Run("non-existent node", func(t *testing.T) {
		_, ok := ns.node(felt.Zero, pathff, true)
		require.False(t, ok)
	})
}

func TestMerge(t *testing.T) {
	old := "old"
	added := "added"
	updated := "updated"

	baseSet := newNodeSet(
		map[trieutils.Path]trienode.TrieNode{
			path1: trienode.NewLeaf([]byte(old)),
		},
		map[trieutils.Path]trienode.TrieNode{
			path2: trienode.NewLeaf([]byte(old)),
		},
		map[felt.Felt]map[trieutils.Path]trienode.TrieNode{
			feltff: {
				path3: trienode.NewLeaf([]byte(old)),
			},
		},
	)

	mergeSet := newNodeSet(
		map[trieutils.Path]trienode.TrieNode{
			path1:  trienode.NewLeaf([]byte(updated)),
			pathff: trienode.NewLeaf([]byte(added)),
		},
		map[trieutils.Path]trienode.TrieNode{
			path2: trienode.NewLeaf([]byte(updated)),
		},
		map[felt.Felt]map[trieutils.Path]trienode.TrieNode{
			feltff: {
				path3: trienode.NewLeaf([]byte(updated)),
				path5: trienode.NewLeaf([]byte(added)),
			},
		},
	)

	baseSet.merge(mergeSet)

	assert.Equal(t, 2, len(baseSet.classNodes))
	assert.Equal(t, []byte(updated), baseSet.classNodes[path1].Blob())
	assert.Equal(t, []byte(added), baseSet.classNodes[pathff].Blob())

	assert.Equal(t, 1, len(baseSet.contractNodes))
	assert.Equal(t, []byte(updated), baseSet.contractNodes[path2].Blob())

	assert.Equal(t, 2, len(baseSet.contractStorageNodes))
	assert.Equal(t, []byte(updated), baseSet.contractStorageNodes[feltff][path3].Blob())
	assert.Equal(t, []byte(added), baseSet.contractStorageNodes[feltff][path5].Blob())

	expectedSize := uint64(
		(len("new") + trieutils.PathSize) + (len("added") + trieutils.PathSize) +
			(len("updated") + trieutils.PathSize) +
			ownerSize + (len("updated-storage") + trieutils.PathSize) + (len("new-storage") + trieutils.PathSize),
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
