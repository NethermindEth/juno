package trie

import (
	"encoding/json"

	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/types"
)

type trieStorer struct {
	store.KVStorer
}

func (kvs *trieStorer) retrieveByP(key *types.Felt) (*types.Felt, *types.Felt, error) {
	// retrieve the args by their pedersen hash
	if value, ok := kvs.Get(append([]byte{0x00}, key.Bytes()...)); !ok {
		// the key should be in the store, if it's not it's an error
		return nil, nil, ErrNotFound
	} else if len(value) != 2*types.FeltLength {
		// the pedersen hash function operates on two felts,
		// so if the value is not 64 bytes it's an error
		return nil, nil, ErrInvalidValue
	} else {
		left := types.BytesToFelt(value[:types.FeltLength])
		right := types.BytesToFelt(value[types.FeltLength:])
		return &left, &right, nil
	}
}

func (kvs *trieStorer) retrieveByH(key *types.Felt) (*Node, error) {
	// retrieve the node by its hash function as defined in the starknet merkle-patricia tree
	if value, ok := kvs.Get(append([]byte{0x01}, key.Bytes()...)); ok {
		// unmarshal the retrived value into the node
		// TODO: use a different serialization format
		n := &Node{}
		err := json.Unmarshal(value, n)
		return n, err
	}
	return &Node{EmptyPath, key}, nil
}

func (kvs *trieStorer) storeByP(key, arg1, arg2 *types.Felt) error {
	value := make([]byte, types.FeltLength*2)
	copy(value[:types.FeltLength], arg1.Bytes())
	copy(value[types.FeltLength:], arg2.Bytes())
	kvs.Put(append([]byte{0x00}, key.Bytes()...), value)
	return nil
}

func (kvs *trieStorer) storeByH(key *types.Felt, node *Node) error {
	value, err := json.Marshal(node)
	if err != nil {
		return err
	}
	kvs.Put(append([]byte{0x01}, key.Bytes()...), value)
	return nil
}
