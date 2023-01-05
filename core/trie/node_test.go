package trie

import (
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
	"github.com/stretchr/testify/assert"
)

func TestNodeMarshalAndUnmarshalBinary(t *testing.T) {
	t.Run("error when marshalling node with nil value", func(t *testing.T) {
		n := new(Node)
		_, err := n.MarshalBinary()

		if err == nil {
			t.Fatal("expected error but got no error")
		}
	})
	t.Run("node with non nil value", func(t *testing.T) {
		value, err := new(felt.Felt).SetRandom()
		if err != nil {
			t.Fatalf("expected no error but got %s", err)
		}
		path1 := bitset.FromWithLength(44, []uint64{44})
		path2 := bitset.FromWithLength(22, []uint64{22})

		tests := [...]struct {
			name string
			node Node
		}{
			{
				name: "node with both children nil",
				node: Node{
					value: value,
					left:  nil,
					right: nil,
				},
			},
			{
				name: "node with left child",
				node: Node{
					value: value,
					left:  path1,
					right: nil,
				},
			},
			{
				name: "node with right child",
				node: Node{
					value: value,
					left:  nil,
					right: path2,
				},
			},
			{
				name: "node with both children (l: path1, r: path2)",
				node: Node{
					value: value,
					left:  path1,
					right: path2,
				},
			},
			{
				name: "node with both children (l: path2, r: path1)",
				node: Node{
					value: value,
					left:  path2,
					right: path1,
				},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				data, err := test.node.MarshalBinary()
				if err != nil {
					t.Fatalf("expected no error but got %s", err)
				}
				unmarshalled := new(Node)
				err = unmarshalled.UnmarshalBinary(data)
				if err != nil {
					t.Fatalf("expected no error but got %s", err)
				}
				if !test.node.Equal(unmarshalled) {
					t.Errorf("expected node: %v but got %v", test.node, unmarshalled)
				}
			})
		}
	})
	t.Run("error when unmarshalling malformed node", func(t *testing.T) {
		malformedNode1 := new([felt.Bytes + 1]byte)
		malformedNode1[felt.Bytes] = 'l'
		malformedNode2 := new([felt.Bytes + 1]byte)
		malformedNode2[felt.Bytes] = 'z'
		tests := [...]struct {
			name string
			node []byte
		}{
			{"input size less than expected felt size", malformedNode1[2:]},
			{"incomplete node", malformedNode1[:]},
			{"unknown child node prefix", malformedNode2[:]},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				if err := new(Node).UnmarshalBinary(test.node); err == nil {
					t.Errorf("expected error but got no error")
				}
			})
		}
	})
	// TODO: Implement and test the following edge cases:
	//	- Unmarshalling a node with multiple left and right children.
	//		Currently the assumption is that the node will only have at max 1 left and/or right
	//		child. However if the UnmarshalBinary is called with multiple left and/or right child
	//		in any order then UnmarshalBinary will still succeed.
}

func TestNodeHash(t *testing.T) {
	// https://github.com/eqlabs/pathfinder/blob/5e0f4423ed9e9385adbe8610643140e1a82eaef6/crates/pathfinder/src/state/merkle_node.rs#L350-L374
	valueBytes, _ := hex.DecodeString("1234ABCD")
	expected, _ := new(felt.Felt).SetString("0x1d937094c09b5f8e26a662d21911871e3cbc6858d55cc49af9848ea6fed4e9")

	node := Node{
		value: new(felt.Felt).SetBytes(valueBytes),
	}
	path := bitset.FromWithLength(6, []uint64{42})

	assert.Equal(t, true, expected.Equal(node.Hash(path)), "TestTrieNode_Hash failed")
}
