package trie

import (
	"errors"

	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/types"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidValue  = errors.New("invalid value")
	ErrUnexistingKey = errors.New("unexisting key")
)

type Trie interface {
	Root() *types.Felt
	Get(key *types.Felt) (*types.Felt, error)
	Put(key *types.Felt, value *types.Felt) error
	Del(key *types.Felt) error
}

type trie struct {
	height int
	root   *types.Felt
	storer *trieStorer
}

// New creates a new trie, pass zero as root hash to initialize an empty trie
func New(kvStorer store.KVStorer, root *types.Felt, height int) Trie {
	return &trie{height, root, &trieStorer{kvStorer}}
}

// Root returns the hash of the root node of the trie.
func (t *trie) Root() *types.Felt {
	return t.root
}

// Get gets the value for a key stored in the trie.
func (t *trie) Get(key *types.Felt) (*types.Felt, error) {
	path := NewPath(t.height, key.Bytes())
	node, _, err := t.get(path, false)
	return node, err
}

// Put inserts a new key/value pair into the trie.
func (t *trie) Put(key *types.Felt, value *types.Felt) error {
	path := NewPath(t.height, key.Bytes())
	_, siblings, err := t.get(path, true)
	if err != nil {
		return err
	}
	return t.put(path, value, siblings)
}

// Del deltes the value associated with the given key.
func (t *trie) Del(key *types.Felt) error {
	return t.Put(key, &types.Felt0)
}

func (t *trie) get(path *Path, withSiblings bool) (*types.Felt, []trieNode, error) {
	// list of siblings we need to hash with to get to the root
	var siblings []trieNode
	if withSiblings {
		siblings = make([]trieNode, t.height)
	}
	curr := t.root // curr is the current node's hash in the traversal
	for walked := 0; walked < t.height && curr.Cmp(EmptyNode.Hash()) != 0; {
		retrieved, err := t.storer.retrieveByH(curr)
		if err != nil {
			return nil, nil, err
		}

		// switch on the type of the node
		switch node := retrieved.(type) {
		case *edgeNode:
			// fmt.Printf("get %3s: %s (%s,%s)\n", path.Prefix(walked).String(), "edge", node.Path().String(), node.Bottom().Hex())

			// longest common prefix of the key and the edge's path
			lcp := node.Path().LongestCommonPrefix(path.Walked(walked))

			if lcp == node.Path().Len() {
				// if the lcp is the length of the path, we need to go down the edge
				// the node we jump to is either a leaf or a binary node, hence its
				// hash is stored in the edge's bottom
				curr = node.Bottom()
			} else {
				// our path diverges with the edge's path
				if withSiblings {
					// we need to collect the node lcp+1 steps down the edge
					if lcp+1 < node.Path().Len() {
						// sibling is still an edge node
						edgePath := node.Path().Walked(lcp + 1)
						siblings[walked+lcp] = &edgeNode{nil, edgePath, node.Bottom()}
					} else if lcp+1 < path.Walked(walked).Len() {
						// sibling is a binary node, we need to retrieve it from the store
						sibling, err := t.storer.retrieveByH(node.Bottom())
						if err != nil {
							return nil, nil, err
						}
						// add sibling to the list of siblings
						siblings[walked+lcp] = sibling
					} else {
						// sibling is a leaf node
						siblings[walked+lcp] = &leafNode{node.Bottom()}
					}
				}

				// we jump to an empty node since we didn't match the path in the edge
				curr = EmptyNode.Hash()
			}

			// we just walk down lcp steps
			walked += lcp

		case *binaryNode:
			// fmt.Printf("get %3s: %s (%s,%s)\n", path.Prefix(walked).String(), "binary", node.leftH.Hex(), node.rightH.Hex())

			var nextH, siblingH *types.Felt
			// walk left or right depending on the bit
			if path.Get(walked) {
				// next is right node
				nextH, siblingH = node.rightH, node.leftH
			} else {
				// next is left node
				nextH, siblingH = node.leftH, node.rightH
			}

			if withSiblings {
				if path.Walked(walked).Len() > 1 {
					// sibling is a binary node, we need to retrieve it from the store
					sibling, err := t.storer.retrieveByH(siblingH)
					if err != nil {
						return nil, nil, err
					}
					// add sibling to the list of siblings
					siblings[walked] = sibling
				} else {
					// sibling is a leaf node
					siblings[walked] = &leafNode{siblingH}
				}
			}

			// get the next node
			curr = nextH
			// increment the walked counter
			walked++
		}
	}
	return curr, siblings, nil
}

// put inserts a node in a given path in the trie.
func (t *trie) put(path *Path, value *types.Felt, siblings []trieNode) error {
	var node trieNode
	node = &leafNode{value}
	// reverse walk the key
	for i := path.Len() - 1; i >= 0; i-- {
		sibling := siblings[i]
		if sibling == nil {
			sibling = EmptyNode
		}

		var left, right trieNode
		if path.Get(i) {
			left, right = sibling, node
		} else {
			left, right = node, sibling
		}

		leftIsEmpty := left.Hash().Cmp(EmptyNode.Hash()) == 0
		rightIsEmpty := right.Hash().Cmp(EmptyNode.Hash()) == 0

		// compute parent
		if leftIsEmpty && rightIsEmpty {
			node = EmptyNode
			// fmt.Printf("put %3s: %s %s\n", path.Prefix(i).String(), "empty", node.Bottom().Hex())
		} else if leftIsEmpty {
			edgePath := NewPath(right.Path().Len()+1, right.Path().Bytes())
			edgePath.Set(0)
			node = &edgeNode{nil, edgePath, right.Bottom()}
			// fmt.Printf("put %3s: %s (%s,%s)\n", path.Prefix(i).String(), "edgeRight", node.Path().String(), node.Bottom().Hex())
		} else if rightIsEmpty {
			edgePath := NewPath(left.Path().Len()+1, left.Path().Bytes())
			node = &edgeNode{nil, edgePath, left.Bottom()}
			// fmt.Printf("put %3s: %s (%s,%s)\n", path.Prefix(i).String(), "edgeLeft", node.Path().String(), node.Bottom().Hex())
		} else {
			if err := t.storer.storeByH(left); err != nil {
				return err
			}
			if err := t.storer.storeByH(right); err != nil {
				return err
			}
			node = &binaryNode{nil, left.Hash(), right.Hash()}
			if err := t.storer.storeByH(node); err != nil {
				return err
			}
			// fmt.Printf("put %3s: %s (%s,%s)\n", path.Prefix(i).String(), "binary", node.(*binaryNode).leftH.Hex(), node.(*binaryNode).rightH.Hex())
		}
	}

	if err := t.storer.storeByH(node); err != nil {
		return err
	}

	t.root = node.Hash()
	// fmt.Printf("trie root after put is: %s\n", t.RootHash().Hex())
	return nil
}
