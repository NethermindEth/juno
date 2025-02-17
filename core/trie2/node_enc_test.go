package trie2

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeEncodingDecoding(t *testing.T) {
	newFelt := func(v uint64) felt.Felt {
		var f felt.Felt
		f.SetUint64(v)
		return f
	}

	// Helper to create a path with specific bits
	newPath := func(bits ...uint8) *trieutils.BitArray {
		path := &trieutils.BitArray{}
		for _, bit := range bits {
			path.AppendBit(path, bit)
		}
		return path
	}

	tests := []struct {
		name    string
		node    node
		pathLen uint8
		maxPath uint8
		wantErr bool
		errMsg  string
	}{
		{
			name: "edge node with value child",
			node: &edgeNode{
				path:  newPath(1, 0, 1),
				child: &valueNode{Felt: newFelt(123)},
			},
			pathLen: 8,
			maxPath: 8,
		},
		{
			name: "edge node with hash child",
			node: &edgeNode{
				path:  newPath(0, 1, 0),
				child: &hashNode{Felt: newFelt(456)},
			},
			pathLen: 3,
			maxPath: 8,
		},
		{
			name: "binary node with two hash children",
			node: &binaryNode{
				children: [2]node{
					&hashNode{Felt: newFelt(111)},
					&hashNode{Felt: newFelt(222)},
				},
			},
			pathLen: 0,
			maxPath: 8,
		},
		{
			name: "binary node with two leaf children",
			node: &binaryNode{
				children: [2]node{
					&valueNode{Felt: newFelt(555)},
					&valueNode{Felt: newFelt(666)},
				},
			},
			pathLen: 7,
			maxPath: 8,
		},
		{
			name:    "value node at max path length",
			node:    &valueNode{Felt: newFelt(999)},
			pathLen: 8,
			maxPath: 8,
		},
		{
			name:    "hash node below max path length",
			node:    &hashNode{Felt: newFelt(1000)},
			pathLen: 7,
			maxPath: 8,
		},
		{
			name: "edge node with empty path",
			node: &edgeNode{
				path:  newPath(),
				child: &hashNode{Felt: newFelt(1111)},
			},
			pathLen: 0,
			maxPath: 8,
		},
		{
			name: "edge node with max length path",
			node: &edgeNode{
				path:  newPath(1, 1, 1, 1, 1, 1, 1, 1),
				child: &valueNode{Felt: newFelt(2222)},
			},
			pathLen: 0,
			maxPath: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode the node
			encoded := nodeToBytes(tt.node)

			// Try to decode
			hash := tt.node.hash(crypto.Pedersen)
			decoded, err := decodeNode(encoded, hash, tt.pathLen, tt.maxPath)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, decoded)

			// Re-encode the decoded node and compare with original encoding
			reEncoded := nodeToBytes(decoded)
			assert.True(t, bytes.Equal(encoded, reEncoded), "re-encoded node doesn't match original encoding")

			// Test specific node type assertions and properties
			switch n := decoded.(type) {
			case *edgeNode:
				original := tt.node.(*edgeNode)
				assert.Equal(t, original.path.String(), n.path.String(), "edge node paths don't match")
			case *binaryNode:
				// Verify both children are present
				assert.NotNil(t, n.children[0], "left child is nil")
				assert.NotNil(t, n.children[1], "right child is nil")
			case *valueNode:
				original := tt.node.(*valueNode)
				assert.True(t, original.Felt.Equal(&n.Felt), "value node felts don't match")
			case *hashNode:
				original := tt.node.(*hashNode)
				assert.True(t, original.Felt.Equal(&n.Felt), "hash node felts don't match")
			}
		})
	}
}

func TestNodeEncodingDecodingBoundary(t *testing.T) {
	// Test empty/nil nodes
	t.Run("nil node encoding", func(t *testing.T) {
		assert.Panics(t, func() { nodeToBytes(nil) })
	})

	// Test with invalid path lengths
	t.Run("invalid path lengths", func(t *testing.T) {
		blob := make([]byte, hashOrValueNodeSize)
		_, err := decodeNode(blob, nil, 255, 8) // pathLen > maxPath
		require.Error(t, err)
	})

	// Test with empty buffer
	t.Run("empty buffer", func(t *testing.T) {
		_, err := decodeNode([]byte{}, nil, 0, 8)
		require.Error(t, err)
	})
}
