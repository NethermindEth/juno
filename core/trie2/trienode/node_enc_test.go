package trienode

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
	newPath := func(bits ...uint8) *trieutils.Path {
		path := &trieutils.Path{}
		for _, bit := range bits {
			path.AppendBit(path, bit)
		}
		return path
	}

	tests := []struct {
		name    string
		node    Node
		pathLen uint8
		maxPath uint8
		wantErr bool
		errMsg  string
	}{
		{
			name: "edge node with value child",
			node: &EdgeNode{
				Path:  newPath(1, 0, 1),
				Child: &ValueNode{Felt: newFelt(123)},
			},
			pathLen: 8,
			maxPath: 8,
		},
		{
			name: "edge node with hash child",
			node: &EdgeNode{
				Path:  newPath(0, 1, 0),
				Child: &HashNode{Felt: newFelt(456)},
			},
			pathLen: 3,
			maxPath: 8,
		},
		{
			name: "binary node with two hash children",
			node: &BinaryNode{
				Children: [2]Node{
					&HashNode{Felt: newFelt(111)},
					&HashNode{Felt: newFelt(222)},
				},
			},
			pathLen: 0,
			maxPath: 8,
		},
		{
			name: "binary node with two leaf children",
			node: &BinaryNode{
				Children: [2]Node{
					&ValueNode{Felt: newFelt(555)},
					&ValueNode{Felt: newFelt(666)},
				},
			},
			pathLen: 7,
			maxPath: 8,
		},
		{
			name:    "value node at max path length",
			node:    &ValueNode{Felt: newFelt(999)},
			pathLen: 8,
			maxPath: 8,
		},
		{
			name:    "hash node below max path length",
			node:    &HashNode{Felt: newFelt(1000)},
			pathLen: 7,
			maxPath: 8,
		},
		{
			name: "edge node with empty path",
			node: &EdgeNode{
				Path:  newPath(),
				Child: &HashNode{Felt: newFelt(1111)},
			},
			pathLen: 0,
			maxPath: 8,
		},
		{
			name: "edge node with max length path",
			node: &EdgeNode{
				Path:  newPath(1, 1, 1, 1, 1, 1, 1, 1),
				Child: &ValueNode{Felt: newFelt(2222)},
			},
			pathLen: 0,
			maxPath: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode the node
			encoded := EncodeNode(tt.node)

			// Try to decode
			hash := tt.node.Hash(crypto.Pedersen)
			decoded, err := DecodeNode(encoded, &hash, tt.pathLen, tt.maxPath)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, decoded)

			// Re-encode the decoded node and compare with original encoding
			reEncoded := EncodeNode(decoded)
			assert.True(t, bytes.Equal(encoded, reEncoded), "re-encoded node doesn't match original encoding")

			// Test specific node type assertions and properties
			switch n := decoded.(type) {
			case *EdgeNode:
				original := tt.node.(*EdgeNode)
				assert.Equal(t, original.Path.String(), n.Path.String(), "edge node paths don't match")
			case *BinaryNode:
				// Verify both children are present
				assert.NotNil(t, n.Children[0], "left child is nil")
				assert.NotNil(t, n.Children[1], "right child is nil")
			case *ValueNode:
				original := tt.node.(*ValueNode)
				assert.True(t, original.Felt.Equal(&n.Felt), "value node felts don't match")
			case *HashNode:
				original := tt.node.(*HashNode)
				assert.True(t, original.Felt.Equal(&n.Felt), "hash node felts don't match")
			}
		})
	}
}

func TestNodeEncodingDecodingBoundary(t *testing.T) {
	// Test empty/nil nodes
	t.Run("nil node encoding", func(t *testing.T) {
		assert.Panics(t, func() { EncodeNode(nil) })
	})

	// Test with invalid path lengths
	t.Run("invalid path lengths", func(t *testing.T) {
		blob := make([]byte, hashOrValueNodeSize)
		_, err := DecodeNode(blob, &felt.Zero, 255, 8) // pathLen > maxPath
		require.Error(t, err)
	})

	// Test with empty buffer
	t.Run("empty buffer", func(t *testing.T) {
		_, err := DecodeNode([]byte{}, &felt.Zero, 0, 8)
		require.Error(t, err)
	})
}
