package trie2

import (
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/crypto"
)

// A tool for hashing nodes in the trie. It supports both sequential and parallel
// hashing modes.
type hasher struct {
	hashFn   crypto.HashFn // The hash function to use
	parallel bool          // Whether to hash binary node children in parallel
}

func newHasher(hash crypto.HashFn, parallel bool) hasher {
	return hasher{
		hashFn:   hash,
		parallel: parallel,
	}
}

// Computes the hash of a node and returns both the hash node and a cached
// version of the original node. If the node already has a cached hash, returns
// that instead of recomputing.
func (h *hasher) hash(n Node) (Node, Node) {
	if hash, _ := n.cache(); hash != nil {
		return hash, n
	}

	switch n := n.(type) {
	case *EdgeNode:
		collapsed, cached := h.hashEdgeChild(n)
		hn := &HashNode{Felt: *collapsed.Hash(h.hashFn)}
		cached.flags.hash = hn
		return hn, cached
	case *BinaryNode:
		collapsed, cached := h.hashBinaryChildren(n)
		hn := &HashNode{Felt: *collapsed.Hash(h.hashFn)}
		cached.flags.hash = hn
		return hn, cached
	case *ValueNode, *HashNode:
		return n, n
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

func (h *hasher) hashEdgeChild(n *EdgeNode) (collapsed, cached *EdgeNode) {
	collapsed, cached = n.copy(), n.copy()

	switch n.Child.(type) {
	case *EdgeNode, *BinaryNode:
		collapsed.Child, cached.Child = h.hash(n.Child)
	}

	return collapsed, cached
}

func (h *hasher) hashBinaryChildren(n *BinaryNode) (collapsed, cached *BinaryNode) {
	collapsed, cached = n.copy(), n.copy()

	if h.parallel { // TODO(weiihann): double check this parallel strategy
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			if n.Children[0] != nil {
				collapsed.Children[0], cached.Children[0] = h.hash(n.Children[0])
			} else {
				collapsed.Children[0], cached.Children[0] = nilValueNode, nilValueNode
			}
		}()

		go func() {
			defer wg.Done()
			if n.Children[1] != nil {
				collapsed.Children[1], cached.Children[1] = h.hash(n.Children[1])
			} else {
				collapsed.Children[1], cached.Children[1] = nilValueNode, nilValueNode
			}
		}()

		wg.Wait()
	} else {
		if n.Children[0] != nil {
			collapsed.Children[0], cached.Children[0] = h.hash(n.Children[0])
		} else {
			collapsed.Children[0], cached.Children[0] = nilValueNode, nilValueNode
		}

		if n.Children[1] != nil {
			collapsed.Children[1], cached.Children[1] = h.hash(n.Children[1])
		} else {
			collapsed.Children[1], cached.Children[1] = nilValueNode, nilValueNode
		}
	}

	return collapsed, cached
}

// Construct trie proofs and returns the collapsed node (i.e. nodes with hash children)
// and the hashed node.
func (h *hasher) proofHash(original Node) (collapsed, hashed Node) {
	switch n := original.(type) {
	case *EdgeNode:
		en, _ := h.hashEdgeChild(n)
		return en, &HashNode{Felt: *en.Hash(h.hashFn)}
	case *BinaryNode:
		bn, _ := h.hashBinaryChildren(n)
		return bn, &HashNode{Felt: *bn.Hash(h.hashFn)}
	default:
		return n, n
	}
}
