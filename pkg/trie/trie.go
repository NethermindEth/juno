package trie

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/types"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidValue = errors.New("invalid value")
)

type Trie struct {
	root   *Node
	storer *trieStorer
}

func NewTrie(kvStorer store.KVStorer, rootHash *types.Felt) (*Trie, error) {
	storer := &trieStorer{kvStorer}
	if root, err := storer.retrieveByH(rootHash); err != nil {
		return nil, err
	} else {
		return &Trie{root, storer}, nil
	}
}

// Get gets the value for a key stored in the trie.
func (t *Trie) Get(key *types.Felt) (*types.Felt, error) {
	// check if root is not empty
	if t.root == nil {
		return nil, nil
	}
	// find the closest match to the key
	curr, _, err := t.findClosestMatch(key)
	if err != nil {
		return nil, err
	}
	// check if we walked down the whole key
	// if length is zero, it's a leaf node
	// - the trie doesn't store empty nodes
	// - with a binary node we can always take one step down the path
	if curr.Length == 0 {
		return &curr.Bottom, nil
	}
	// the key is not in the trie
	return nil, nil
}

// Put inserts a new key/value pair into the trie.
func (t *Trie) Put(key *types.Felt, value *types.Felt) error {
	// node := &Node{
	// 	Length: len(key.Bytes()) * 8,
	// 	Path:   *key,
	// 	Bottom: *value,
	// }
	// // walk up the path from the leaf node
	// for i := types.FeltBitLen; i >= 0; i-- {
	// 	var left, right *Node
	// 	// if the bit is 0, we came down the left path
	// 	siblingKey := &*key
	// 	siblingKey.SetBit(uint(i), ^key.Bit(uint(i)))
	// 	var (
	// 		sibling *Node
	// 		err     error
	// 	)
	// 	if sibling, err = t.Get(siblingKey); err != nil {
	// 		// there was an error retrieving the sibling node
	// 		return err
	// 	} else if right == nil {
	// 		// there is no sibling node, so assing (0,0,0) to it
	// 		right = &Node{
	// 			Length: 0,
	// 			Path:   types.Felt0,
	// 			Bottom: types.Felt0,
	// 		}
	// 	}
	// 	if key.Bit(uint(i)) == 0 {
	// 		// if the bit is 0, we came down the left path
	// 		right, left = node, sibling
	// 	} else {
	// 		// if the bit is 1, we came down the right path
	// 		left, right = node, sibling
	// 	}
	//
	// }

	// the key is not in the trie
	return nil
}

func (t *Trie) findClosestMatch(key *types.Felt) (*Node, []*Node, error) {
	proof := make([]*Node, 0)
	walked := 0    // steps we have taken so far
	curr := t.root // curr is the current node in the traversal
	for walked < types.FeltBitLen {
		if curr.Length == 0 {
			// node is a binary node or an empty node
			if bytes.Compare(curr.Bottom.Bytes(), types.Felt0.Bytes()) == 0 {
				// node is an empty node (0,0,0)
				// if we haven't matched the whole key yet it's because it's not in the trie
				// NOTE: this should not happen, empty nodes are not stored
				//       and are never reachable from the root
				// TODO: After this is debugged and we make sure it never happens
				//       we can remove the panic
				panic("reached an empty node while traversing the trie")
			}

			// node is a binary node (0,0,h(H(left),H(right)))
			// retrieve the left and right nodes
			// by reverting the pedersen hash function
			leftH, rightH, err := t.storer.retrieveByP(&curr.Bottom)
			if err != nil {
				return nil, nil, err
			}

			var next, sibling *types.Felt
			// walk left or right depending on the bit
			if key.Bit(uint(walked)) == 0 {
				// next is left node
				next, sibling = leftH, rightH
			} else {
				// next is right node
				next, sibling = rightH, leftH
			}

			// retrieve the next node from the store
			if curr, err = t.storer.retrieveByH(next); err != nil {
				return nil, nil, err
			}

			// retrieve the sibling node from the store
			if s, err := t.storer.retrieveByH(sibling); err != nil {
				return nil, nil, err
			} else {
				// append the sibling to the merkle proof
				proof = append(proof, s)
			}

			walked += 1 // we took one node
		} else if curr.isPrefix(key) {
			// node is an edge node and matches the key
			// since curr is an edge, the bottom of curr is actually
			// the bottom of the node it links to after walking down its path
			// this node that curr links to has to be either a binary node or a leaf,
			// hence its path and length are zero
			curr := &Node{0, types.Felt0, curr.Bottom}
			proof = append(proof, curr) // append the edge node to the proof
			walked += curr.Length       // we jumped a path of length `curr.length`
		} else {
			// node length is greater than zero but its path diverges from ours,
			// this means that the key we are looking for is not in the trie
			// break the loop, otherwise we would get stuck here
			// this node will be returned as the closest match outside the loop
			proof = append(proof, curr)
			break
		}
	}
	return curr, proof, nil
}

type trieStorer struct {
	store.KVStorer
}

func (kvs *trieStorer) retrieveByP(key *types.Felt) (*types.Felt, *types.Felt, error) {
	// retrieve the args by their pedersen hash
	if value, ok := kvs.Get(key.Bytes()); !ok {
		// the key should be in the store, if it's not it's an error
		return nil, nil, ErrNotFound
	} else if len(value) != 64 {
		// the pedersen hash function operates on two felts,
		// so if the value is not 64 bytes it's an error
		return nil, nil, ErrInvalidValue
	} else {
		left := types.BytesToFelt(value[:32])
		right := types.BytesToFelt(value[32:])
		return &left, &right, nil
	}
}

func (kvs *trieStorer) retrieveByH(key *types.Felt) (*Node, error) {
	// retrieve the node by its hash function as defined in the starknet merkle-patricia tree
	if value, ok := kvs.Get(key.Bytes()); ok {
		// unmarshal the retrived value into the node
		// TODO: use a different serialization format
		n := &Node{}
		err := json.Unmarshal(value, n)
		return n, err
	}
	// the key should be in the store, if it's not it's an error
	return nil, ErrNotFound
}
