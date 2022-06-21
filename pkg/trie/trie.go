package trie

import (
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

// New creates a new trie, pass zero as root hash to initialize an empty trie
func New(kvStorer store.KVStorer, rootHash *types.Felt, height int) (*trie, error) {
	storer := &trieStorer{kvStorer}
	if root, err := storer.retrieveByH(rootHash); err != nil {
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
	return leaf.Bottom, err
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
	return t.Put(key, &types.Felt0)
}

func (t *trie) get(path *Path) (*Node, []*Node, error) {
	// list of siblings we need to hash with to get to the root
	siblings := make([]*Node, t.height)
	curr := t.root // curr is the current node in the traversal
	for walked := 0; walked < t.height && !curr.IsEmpty(); {
		if curr.Path.Len() == 0 {
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

			// retrieve the sibling from the store, probably good to add a flag to avoid this
			if siblings[walked], err = t.storer.retrieveByH(sibling); err != nil {
				return nil, nil, err
			}
			// retrieve the next node from the store
			if curr, err = t.storer.retrieveByH(next); err != nil {
				return nil, nil, err
			}

			walked += 1 // we took one node
			continue
		}

		// longest common prefix of the key and the node
		lcp := curr.Path.LongestCommonPrefix(path.Walked(walked))

		if lcp == 0 {
			// since we haven't matched the whole key yet, it's not in the trie
			// sibling is the node going one step down the node's path
			siblings[walked] = &Node{curr.Path.Walked(1), curr.Bottom}
			curr = EmptyNode // to be consistent with the meaning of `curr`
		} else {
			// walk down the path of length `lcp`
			curr = &Node{curr.Path.Walked(lcp), curr.Bottom}
		}

		walked += lcp
	}
	return curr, siblings, nil
}

func (t *trie) put(path *Path, node *Node, siblings []*Node) error {
	// reverse walk the key
	for i := path.Len() - 1; i >= 0; i-- {
		sibling := siblings[i]
		if sibling == nil {
			sibling = EmptyNode
		}

		var left, right *Node
		if path.Get(i) {
			left, right = sibling, node
		} else {
			left, right = node, sibling
		}

		// compute parent
		if left.IsEmpty() && right.IsEmpty() {
			node = EmptyNode
		} else if left.IsEmpty() {
			path := NewPath(right.Path.Len()+1, right.Path.Bytes())
			path.Set(0)
			node = &Node{path, right.Bottom}
		} else if right.IsEmpty() {
			path := NewPath(left.Path.Len()+1, left.Path.Bytes())
			node = &Node{path, left.Bottom}
		} else {
			leftH, err := t.computeH(left)
			if err != nil {
				return err
			}
			rightH, err := t.computeH(right)
			if err != nil {
				return err
			}
			// create the binary node
			bottom, err := t.computeP(leftH, rightH)
			if err != nil {
				return err
			}
			node = &Node{EmptyPath, bottom}
		}
	}
	t.root = node
	_, err := t.computeH(t.root)
	return err
}

// computeH computes the hash of the node and stores it in the store
func (t *trie) computeH(node *Node) (*types.Felt, error) {
	// compute the hash of the node
	h := node.Hash()
	// only store hash of edge nodes, as the hash function
	// for non-edge nodes is `f(x) = x`
	if node.IsEdge() {
		// store the hash of the node
		if err := t.storer.storeByH(h, node); err != nil {
			return nil, err
		}
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
