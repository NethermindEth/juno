package trienode

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/core/types/felt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeEncodingDecoding(t *testing.T) {
	newFelt := func(val uint64) felt.Felt {
		var f felt.Felt
		f.SetUint64(val)
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
				Path: newPath(1, 0, 1),
				Child: func() *ValueNode {
					f := newFelt(123)
					return (*ValueNode)(&f)
				}(),
			},
			pathLen: 8,
			maxPath: 8,
		},
		{
			name: "edge node with hash child",
			node: &EdgeNode{
				Path: newPath(0, 1, 0),
				Child: func() *HashNode {
					f := newFelt(456)
					return (*HashNode)(&f)
				}(),
			},
			pathLen: 3,
			maxPath: 8,
		},
		{
			name: "binary node with two hash children",
			node: &BinaryNode{
				Children: [2]Node{
					func() *HashNode {
						f := newFelt(111)
						return (*HashNode)(&f)
					}(),
					func() *HashNode {
						f := newFelt(222)
						return (*HashNode)(&f)
					}(),
				},
			},
			pathLen: 0,
			maxPath: 8,
		},
		{
			name: "binary node with two leaf children",
			node: &BinaryNode{
				Children: [2]Node{
					func() *ValueNode {
						f := newFelt(555)
						return (*ValueNode)(&f)
					}(),
					func() *ValueNode {
						f := newFelt(666)
						return (*ValueNode)(&f)
					}(),
				},
			},
			pathLen: 7,
			maxPath: 8,
		},
		{
			name: "value node at max path length",
			node: func() *ValueNode {
				f := newFelt(999)
				return (*ValueNode)(&f)
			}(),
			pathLen: 8,
			maxPath: 8,
		},
		{
			name: "hash node below max path length",
			node: func() *HashNode {
				f := newFelt(1000)
				return (*HashNode)(&f)
			}(),
			pathLen: 7,
			maxPath: 8,
		},
		{
			name: "edge node with empty path",
			node: &EdgeNode{
				Path: newPath(),
				Child: func() *HashNode {
					f := newFelt(1111)
					return (*HashNode)(&f)
				}(),
			},
			pathLen: 0,
			maxPath: 8,
		},
		{
			name: "edge node with max length path",
			node: &EdgeNode{
				Path: newPath(1, 1, 1, 1, 1, 1, 1, 1),
				Child: func() *ValueNode {
					f := newFelt(2222)
					return (*ValueNode)(&f)
				}(),
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
				if felt.Felt(*original) != felt.Felt(*n) {
					t.Errorf("value node mismatch: got %v, want %v", felt.Felt(*n), felt.Felt(*original))
				}
			case *HashNode:
				original := tt.node.(*HashNode)
				if felt.Felt(*original) != felt.Felt(*n) {
					t.Errorf("hash node mismatch: got %v, want %v", felt.Felt(*n), felt.Felt(*original))
				}
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
