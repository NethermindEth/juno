package trienode

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/stretchr/testify/require"
)

func TestNodeSet(t *testing.T) {
	t.Run("new node set", func(t *testing.T) {
		ns := NewNodeSet(felt.Zero)
		require.Equal(t, felt.Zero, ns.Owner)
		require.Empty(t, ns.Nodes)
		require.Zero(t, ns.updates)
		require.Zero(t, ns.deletes)
	})

	t.Run("add nodes", func(t *testing.T) {
		ns := NewNodeSet(felt.Zero)

		// Add a regular node
		key1 := trieutils.NewBitArray(8, 0xFF)
		node1 := NewLeaf([]byte{1, 2, 3})
		ns.Add(key1, node1)
		require.Equal(t, 1, ns.updates)
		require.Equal(t, 0, ns.deletes)

		// Add a deleted node
		key2 := trieutils.NewBitArray(8, 0xAA)
		node2 := NewDeleted(false)
		ns.Add(key2, node2)
		require.Equal(t, 1, ns.updates)
		require.Equal(t, 1, ns.deletes)

		// Verify nodes are stored correctly
		require.Equal(t, node1, ns.Nodes[key1])
		require.Equal(t, node2, ns.Nodes[key2])
	})

	t.Run("merge sets", func(t *testing.T) {
		ns1 := NewNodeSet(felt.Zero)
		ns2 := NewNodeSet(felt.Zero)

		// Add nodes to first set
		key1 := trieutils.NewBitArray(8, 0xFF)
		node1 := NewLeaf([]byte{1, 2, 3})
		ns1.Add(key1, node1)

		// Add nodes to second set
		key2 := trieutils.NewBitArray(8, 0xAA)
		node2 := NewDeleted(false)
		ns2.Add(key2, node2)

		// Merge sets
		err := ns1.MergeSet(ns2)
		require.NoError(t, err)

		// Verify merged state
		require.Equal(t, 2, len(ns1.Nodes))
		require.Equal(t, node1, ns1.Nodes[key1])
		require.Equal(t, node2, ns1.Nodes[key2])
		require.Equal(t, 1, ns1.updates)
		require.Equal(t, 1, ns1.deletes)
	})

	t.Run("merge with different owners", func(t *testing.T) {
		owner1 := new(felt.Felt).SetUint64(123)
		owner2 := new(felt.Felt).SetUint64(456)
		ns1 := NewNodeSet(*owner1)
		ns2 := NewNodeSet(*owner2)

		err := ns1.MergeSet(ns2)
		require.Error(t, err)
	})

	t.Run("merge map", func(t *testing.T) {
		owner := new(felt.Felt).SetUint64(123)
		ns := NewNodeSet(*owner)

		// Create a map to merge
		nodes := make(map[trieutils.Path]TrieNode)
		key1 := trieutils.NewBitArray(8, 0xFF)
		node1 := NewLeaf([]byte{1, 2, 3})
		nodes[key1] = node1

		// Merge map
		err := ns.Merge(*owner, nodes)
		require.NoError(t, err)

		// Verify merged state
		require.Equal(t, 1, len(ns.Nodes))
		require.Equal(t, node1, ns.Nodes[key1])
		require.Equal(t, 1, ns.updates)
		require.Equal(t, 0, ns.deletes)
	})

	t.Run("foreach", func(t *testing.T) {
		ns := NewNodeSet(felt.Zero)

		// Add nodes in random order
		keys := []trieutils.Path{
			trieutils.NewBitArray(8, 0xFF),
			trieutils.NewBitArray(8, 0xAA),
			trieutils.NewBitArray(8, 0x55),
		}
		for _, key := range keys {
			ns.Add(key, NewLeaf([]byte{1}))
		}

		t.Run("ascending order", func(t *testing.T) {
			var visited []trieutils.Path
			_ = ns.ForEach(false, func(key trieutils.Path, node TrieNode) error {
				visited = append(visited, key)
				return nil
			})

			// Verify ascending order
			for i := 1; i < len(visited); i++ {
				require.True(t, visited[i-1].Cmp(&visited[i]) < 0)
			}
		})

		t.Run("descending order", func(t *testing.T) {
			var visited []trieutils.Path
			_ = ns.ForEach(true, func(key trieutils.Path, node TrieNode) error {
				visited = append(visited, key)
				return nil
			})

			// Verify descending order
			for i := 1; i < len(visited); i++ {
				require.True(t, visited[i-1].Cmp(&visited[i]) > 0)
			}
		})
	})
}
