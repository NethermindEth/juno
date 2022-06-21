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
	ErrNotFound      = errors.New("not found")
	ErrInvalidValue  = errors.New("invalid value")
	ErrUnexistingKey = errors.New("unexisting key")
)

type Trie interface {
	RootHash() *types.Felt
	Get(key *types.Felt) (*types.Felt, error)
	Put(key *types.Felt, value *types.Felt) error
	Del(key *types.Felt) error
}

type trie struct {
	root   *Node
	storer *trieStorer
	height int
}

func New(kvStorer store.KVStorer, rootHash *types.Felt, height int) (*trie, error) {
	storer := &trieStorer{kvStorer}
	if rootHash == nil {
		return &trie{nil, storer, height}, nil
	} else if root, err := storer.retrieveByH(rootHash); err != nil {
		return nil, err
	} else {
		return &trie{root, storer, height}, nil
	}
}

// RootHash returns the hash of the root node of the trie.
func (t *trie) RootHash() *types.Felt {
	if t.root == nil {
		return &types.Felt0
	}
	return t.root.Hash()
}

// Get gets the value for a key stored in the trie.
func (t *trie) Get(key *types.Felt) (*types.Felt, error) {
	path := NewPath(t.height, key.Bytes())
	leaf, _, err := t.get(path)
	if err != nil {
		return nil, err
	}
	if leaf != nil {
		return leaf.Bottom, nil
	}
	return nil, nil
}

// Put inserts a new key/value pair into the trie.
func (t *trie) Put(key *types.Felt, value *types.Felt) error {
	path := NewPath(t.height, key.Bytes())
	_, siblings, err := t.get(path)
	if err != nil {
		return err
	}
	return t.put(path, &Node{EmptyPath, value}, siblings)
}

// Del deltes the value associated with the given key.
func (t *trie) Del(key *types.Felt) error {
	path := NewPath(t.height, key.Bytes())
	if leaf, siblings, err := t.get(path); err != nil {
		return err
	} else if leaf == nil {
		return ErrUnexistingKey
	} else {
		return t.put(path, nil, siblings)
	}
}

func (t *trie) get(path *Path) (*Node, []*types.Felt, error) {
	// list of siblings we need to hash with to get to the root
	siblings := make([]*types.Felt, t.height)
	curr := t.root // curr is the current node in the traversal
	for walked := 0; walked < t.height && curr != nil; {
		if curr.Path.Len() == 0 {
			// node is a binary node or an empty node
			if bytes.Equal(curr.Bottom.Bytes(), types.Felt0.Bytes()) {
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
			leftH, rightH, err := t.storer.retrieveByP(curr.Bottom)
			if err != nil {
				return nil, nil, err
			}

			var next, sibling *types.Felt
			// walk left or right depending on the bit
			if path.Get(walked) {
				// next is right node
				next, sibling = rightH, leftH
			} else {
				// next is left node
				next, sibling = leftH, rightH
			}

			siblings[walked] = sibling
			// retrieve the next node from the store
			if curr, err = t.storer.retrieveByH(next); err != nil {
				return nil, nil, err
			}

			walked += 1 // we took one node
			continue
		}

		// longest common prefix of the key and the node
		lcp := curr.Path.longestCommonPrefix(path.Walked(walked))

		if lcp == 0 {
			// since we haven't matched the whole key yet, it's not in the trie
			// sibling is the node going one step down the node's path
			siblings[walked] = (&Node{curr.Path.Walked(1), curr.Bottom}).Hash()
			curr = nil // to be consistent with the meaning of `curr`
		} else {
			// walk down the path of length `lcp`
			curr = &Node{curr.Path.Walked(lcp), curr.Bottom}
		}

		walked += lcp
	}
	return curr, siblings, nil
}

func (t *trie) put(path *Path, node *Node, siblings []*types.Felt) (err error) {
	var hash *types.Felt
	if node != nil {
		// insert the node into the kvStore and keep its hash
		if hash, err = t.computeH(node); err != nil {
			return err
		}
	}
	// reverse walk the key
	for i := path.Len() - 1; i >= 0; i-- {
		// if we have a sibling for this bit, we insert a binary node
		if sibling := siblings[i]; sibling != nil {
			if node == nil {
				sibling, err := t.storer.retrieveByH(sibling)
				if err != nil {
					return err
				}
				edgePath := NewPath(sibling.Path.Len()+1, sibling.Path.Bytes())
				if !path.Get(i) {
					edgePath.Set(0)
				}
				node = &Node{edgePath, sibling.Bottom}
			} else {
				var left, right *types.Felt
				if path.Get(i) {
					left, right = sibling, hash
				} else {
					left, right = hash, sibling
				}
				// create the binary node
				bottom, err := t.computeP(left, right)
				if err != nil {
					return err
				}
				node = &Node{EmptyPath, bottom}
			}
		} else if node != nil {
			// otherwise we just insert an edge node
			edgePath := NewPath(node.Path.Len()+1, node.Path.Bytes())
			if path.Get(i) {
				edgePath.Set(0)
			}
			node = &Node{edgePath, node.Bottom}
		} else {
			continue
		}
		// insert the node into the kvStore and keep its hash
		hash, err = t.computeH(node)
		if err != nil {
			return err
		}
	}
	t.root = node
	return nil
}

// computeH computes the hash of the node and stores it in the store
func (t *trie) computeH(node *Node) (*types.Felt, error) {
	// compute the hash of the node
	h := node.Hash()
	// store the hash of the node
	if err := t.storer.storeByH(h, node); err != nil {
		return nil, err
	}
	return h, nil
}

// computeP computes the pedersen hash of the felts and stores it in the store
func (t *trie) computeP(arg1, arg2 *types.Felt) (*types.Felt, error) {
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
	// the key should be in the store, if it's not it's an error
	return nil, ErrNotFound
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
