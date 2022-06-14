package trie

import (
	"bytes"
	"errors"

	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/types"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidValue = errors.New("invalid value")
)

type Trie struct {
	root   *node
	storer *trieStorer
}

func NewTrie(kvStorer store.KVStorer, rootHash []byte) (*Trie, error) {
	root := &node{}
	storer := &trieStorer{kvStorer}
	if err := storer.retrieveByH(rootHash, root); err != nil {
		return nil, err
	}
	return &Trie{root, storer}, nil
}

// Get gets the value for a key stored in the trie.
func (t *Trie) Get(key *types.Felt) (*types.Felt, error) {
	// curr is the current node in the traversal
	curr := &*t.root // the `&*` part is done to prevent heap allocations
	for walked := 0; walked < len(key.Bytes()); walked++ {
		if curr.length == 0 {
			// node is a binary node or an empty node
			if bytes.Compare(curr.bottom.Bytes(), types.Felt0.Bytes()) == 0 {
				// node is an empty node (0,0,0)
				// if we haven't matched the whole key yet it's because it's not in the trie
				return nil, nil
			}

			// node is a binary node (0,0,h(H(left),H(right)))
			// retrieve the left and right nodes
			// by reverting the pedersen hash function
			bottom, err := t.storer.retrieveByP(curr.bottom.Bytes())
			if err != nil {
				return nil, err
			}

			var next []byte
			// walk left or right depending on the bit
			if key.Big().Bit(walked) == 0 {
				// next is left node
				next = bottom[:32]
			} else {
				// next is right node
				next = bottom[32:]
			}

			// retrieve the corresponding node from the store
			// unmarshal the retrieved value into curr
			if err := t.storer.retrieveByH(next, curr); err != nil {
				return nil, err
			}
		} else if curr.IsPrefix(key) {
			// node is an edge node and matches the key
			// since curr is an edge, the bottom of curr is actually
			// the bottom of the node it links to after walking down its path
			// this node that curr links to has to be either a binary node or a leaf,
			// hence its path and length are zero
			// we just assign length and path to `curr` here as if we had walked down
			// the path indicated by the edge
			curr.length, curr.path = 0, types.Felt0
		} else {
			// node length is greater than zero but its path is diverges from ours,
			// this means that the key we are looking for is not in the trie
			return nil, nil
		}
	}
	return &curr.bottom, nil
}

// Put inserts a new key/value pair into the trie.
func (t *Trie) Put(key *types.Felt, value *types.Felt) error {
	return nil
}

type trieStorer struct {
	store.KVStorer
}

func (kvs *trieStorer) retrieveByP(key []byte) ([]byte, error) {
	// retrieve the args by their pedersen hash
	if value, ok := kvs.Get(key); !ok {
		// the key should be in the store, if it's not it's an error
		return nil, ErrNotFound
	} else if len(value) != 64 {
		// the pedersen hash function operates on two felts,
		// so if the value is not 64 bytes it's an error
		return nil, ErrInvalidValue
	} else {
		return value, nil
	}
}

func (kvs *trieStorer) retrieveByH(key []byte, n *node) error {
	// retrieve the node by its hash function as defined in the starknet merkle-patricia tree
	if value, ok := kvs.Get(key); !ok {
		// the key should be in the store, if it's not it's an error
		return ErrNotFound
	} else {
		// unmarshal the retrived value into the node
		// TODO: use a different serialization format
		return n.UnmarshalJSON(value)
	}
}
