package trie

import (
	"fmt"

	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/types"
)

type trieStorer struct {
	store.KVStorer
}

func (t *trieStorer) storeByH(node trieNode) error {
	marshaled, err := node.MarshalBinary()
	if err != nil {
		return err
	}
	t.Put(append([]byte("patricia_node:"), node.Hash().Bytes()...), marshaled)
	return nil
}

func (t *trieStorer) retrieveByH(hash *types.Felt) (trieNode, error) {
	marshaled, ok := t.Get(append([]byte("patricia_node:"), hash.Bytes()...))
	if !ok {
		return nil, fmt.Errorf("not found: %s", hash.Hex())
	}
	var node trieNode
	if len(marshaled) == 2*types.FeltLength {
		node = new(binaryNode)
	} else if len(marshaled) == 2*types.FeltLength+1 {
		node = new(edgeNode)
	} else {
		return nil, ErrInvalidValue
	}
	err := node.UnmarshalBinary(marshaled)
	return node, err
}
