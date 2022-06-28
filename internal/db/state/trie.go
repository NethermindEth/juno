package state

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/pkg/collections"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"
)

func (m *Manager) GetTrieNode(hash *types.Felt) (trie.TrieNode, error) {
	// Search on the database
	rawNode, err := m.trieDatabase.Get(hash.Bytes())
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
		leftH := types.BytesToFelt(binaryNode.GetLeftH())
		rightH := types.BytesToFelt(binaryNode.GetRightH())
		return &trie.BinaryNode{
			LeftH:  &leftH,
			RightH: &rightH,
		}, nil
	}
	if edgeNode := node.GetEdgeNode(); edgeNode != nil {
		// Return edge node
		path := collections.NewBitSet(int(edgeNode.GetLength()), edgeNode.GetPath())
		bottom := types.BytesToFelt(edgeNode.GetBottom())
		return trie.NewEdgeNode(path, &bottom), nil
	}
	// Return error if unknown node type
	return nil, errors.New("unknown node type")
}

func (m *Manager) StoreTrieNode(node trie.TrieNode) error {
	key := node.Hash().Bytes()
	switch node := node.(type) {
	case *trie.BinaryNode:
		rawNode, err := proto.Marshal(&BinaryNode{
			LeftH:  node.LeftH.Bytes(),
			RightH: node.RightH.Bytes(),
		})
		if err != nil {
			panic(err)
		}
		return m.trieDatabase.Put(key, rawNode)
	case *trie.EdgeNode:
		rawNode, err := proto.Marshal(&EdgeNode{
			Length: uint32(node.Path().Len()),
			Path:   node.Path().Bytes(),
			Bottom: node.Bottom().Bytes(),
		})
		if err != nil {
			panic(err)
		}
		return m.trieDatabase.Put(key, rawNode)
	default:
		panic(fmt.Sprintf("unknown node type: %T", node))
	}
}
