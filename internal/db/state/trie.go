package state

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/pkg/collections"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/trie"
	"google.golang.org/protobuf/proto"
)

func (m *Manager) GetTrieNode(hash *felt.Felt) (trie.TrieNode, error) {
	// Search on the database
	rawNode, err := m.stateDatabase.Get(hash.ByteSlice())
	if err != nil {
		panic(err)
	}
	if rawNode == nil {
		// Return nil if not found
		return nil, nil
	}
	// Unmarshal to protobuf struct
	var node TrieNode
	err = proto.Unmarshal(rawNode, &node)
	if err != nil {
		return nil, err
	}
	if binaryNode := node.GetBinaryNode(); binaryNode != nil {
		// Return binary node
		return &trie.BinaryNode{
			LeftH:  new(felt.Felt).SetBytes(binaryNode.GetLeftH()),
			RightH: new(felt.Felt).SetBytes(binaryNode.GetRightH()),
		}, nil
	}
	if edgeNode := node.GetEdgeNode(); edgeNode != nil {
		// Return edge node
		return trie.NewEdgeNode(
			collections.NewBitSet(int(edgeNode.GetLength()), edgeNode.GetPath()),
			new(felt.Felt).SetBytes(edgeNode.GetBottom()),
		), nil
	}
	// Return error if unknown node type
	return nil, errors.New("unknown node type")
}

func (m *Manager) StoreTrieNode(node trie.TrieNode) error {
	key := node.Hash().ByteSlice()
	var pbTrieNode isTrieNode_Node
	switch node := node.(type) {
	case *trie.BinaryNode:
		pbTrieNode = &TrieNode_BinaryNode{
			BinaryNode: &BinaryNode{
				LeftH:  node.LeftH.ByteSlice(),
				RightH: node.RightH.ByteSlice(),
			},
		}
	case *trie.EdgeNode:
		pbTrieNode = &TrieNode_EdgeNode{
			&EdgeNode{
				Length: uint32(node.Path().Len()),
				Path:   node.Path().Bytes(),
				Bottom: node.Bottom().ByteSlice(),
			},
		}
	default:
		panic(fmt.Sprintf("unknown node type: %T", node))
	}
	rawNode, err := proto.Marshal(&TrieNode{
		Node: pbTrieNode,
	})
	if err != nil {
		return err
	}
	return m.stateDatabase.Put(key, rawNode)
}
