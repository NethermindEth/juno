package trie

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
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
		if err == ErrNotFound {
			return &Trie{nil, storer}, nil
		}
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
				return nil, err
			}

			var next *types.Felt
			// walk left or right depending on the bit
			if key.Bit(uint(walked)) == 0 {
				// next is left node
				next = leftH
			} else {
				// next is right node
				next = rightH
			}

			// retrieve the next node from the store
			if curr, err = t.storer.retrieveByH(next); err != nil {
				return nil, err
			}

			walked += 1 // we took one node
		} else if curr.longestCommonPrefix(key, walked) == curr.Length {
			// node is an edge node and matches the key
			// since curr is an edge, the bottom of curr is actually
			// the bottom of the node it links to after walking down its path
			// this node that curr links to has to be either a binary node or a leaf,
			// hence its path and length are zero
			curr := &Node{0, types.Felt0, curr.Bottom}
			walked += curr.Length // we jumped a path of length `curr.length`
		} else {
			// node length is greater than zero but its path diverges from ours,
			// this means that the key we are looking for is not in the trie
			// break the loop, otherwise we would get stuck here
			// this node will be returned as the closest match outside the loop
			return nil, nil
		}
	}
	return &curr.Bottom, nil
}

// Put inserts a new key/value pair into the trie.
func (t *Trie) Put(key *types.Felt, value *types.Felt) error {
	siblings := make(map[int]*types.Felt)
	curr := t.root // curr is the current node in the traversal
	for walked := 0; walked < types.FeltBitLen; {
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
				return err
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

			siblings[walked] = sibling
			// retrieve the next node from the store
			if curr, err = t.storer.retrieveByH(next); err != nil {
				return err
			}

			walked += 1 // we took one node
			continue
		}

		// longest common prefix of the key and the node
		lcp := curr.longestCommonPrefix(key, walked)

		// TODO: this explanation is from previous implementation,
		//       we should adapt it to the new implementation. It is
		//       not completely wrong, but it's not entirely accurate.
		//
		// node length is greater than zero
		//
		// consider the following:
		// let sib = node with key `key[:walked+lcp+neg(key[walked+lcp+1])]`
		//           where neg(x) returns 0 if x is 1 and 1 otherwise
		// the node with `sib` is our sibling from step `walked+lcp`
		// since curr is an edge node and sib is just a node in the path from curr to wherever curr links to,
		// in case `sib` is a binary node or a leaf, it is of the form (0,0,curr.bottom)
		// in case `sib` is an edge node, it is of the form (curr.length-lcp,curr.path[lcp:],curr.bottom)
		//
		// in order to get `sib` easily we will just walk down lcp steps
		// if lcp was in fact the whole pathbut still haven't walked down the whole key,
		// we would have come down to a binary node, which are handled above

		// node is an edge node and matches the key
		// since curr is an edge, the bottom of curr is actually
		// the bottom of the node it links to after walking down its path
		// this node that curr links to has to be either a binary node or a leaf,
		// hence its path and length are zero

		if lcp == 0 {
			// since we haven't matched the whole key yet, it's not in the trie
			// sibling is the node going one step down the node's path
			siblings[walked] = (&Node{curr.Length - 1, curr.Path, curr.Bottom}).Hash()
			// break the loop, otherwise we would get stuck here
			break
		}

		// walk down the path of length `lcp`
		curr = &Node{curr.Length - lcp, curr.Path, curr.Bottom}
		walked += lcp
	}

	curr = &Node{0, types.Felt0, *value} // starting from the leaf
	// insert the node into the kvStore and keep its hash
	hash, err := t.computeH(curr)
	if err != nil {
		return err
	}
	// reverse walk the key
	for i := types.FeltBitLen - 1; i >= 0; i-- {
		// if we have a sibling for this bit, we insert a binary node
		if sibling, ok := siblings[i]; ok {
			var left, right *types.Felt
			if key.Bit(uint(i)) == 1 {
				left, right = sibling, hash
			} else {
				left, right = hash, sibling
			}
			// create the binary node
			bottom, err := t.computeP(left, right)
			if err != nil {
				return err
			}
			curr = &Node{0, types.Felt0, *bottom}
		} else {
			// otherwise we just insert an edge node
			path := curr.Path
			path.SetBit(uint(types.FeltBitLen-curr.Length), uint(1))
			curr = &Node{curr.Length + 1, path, curr.Bottom}
		}
		// insert the node into the kvStore and keep its hash
		hash, err = t.computeH(curr)
		if err != nil {
			return err
		}
	}

	t.root = curr
	return nil
}

// computeH computes the hash of the node and stores it in the store
func (t *Trie) computeH(node *Node) (*types.Felt, error) {
	// compute the hash of the node
	h := node.Hash()
	// store the hash of the node
	if err := t.storer.storeByH(h, node); err != nil {
		return nil, err
	}
	return h, nil
}

// computeP computes the pedersen hash of the felts and stores it in the store
func (t *Trie) computeP(arg1, arg2 *types.Felt) (*types.Felt, error) {
	// compute the pedersen hash of the node
	p := types.BigToFelt(pedersen.Digest(arg1.Big(), arg2.Big()))
	// store the pedersen hash of the node
	if err := t.storer.storeByP(&p, arg1, arg2); err != nil {
		return nil, err
	}
	return &p, nil
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
	if key == nil {
		return nil, ErrNotFound
	}
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

func (kvs *trieStorer) storeByP(key, arg1, arg2 *types.Felt) error {
	value := make([]byte, 64)
	copy(value[:32], arg1.Bytes())
	copy(value[32:], arg2.Bytes())
	kvs.Put(key.Bytes(), value)
	return nil
}

func (kvs *trieStorer) storeByH(key *types.Felt, node *Node) error {
	value, err := json.Marshal(node)
	if err != nil {
		return err
	}
	kvs.Put(key.Bytes(), value)
	return nil
}
