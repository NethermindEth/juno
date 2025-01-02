package trie2

import (
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/crypto"
)

// hasher handles node hashing for the trie. It supports both sequential and parallel
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

// hash computes the hash of a node and returns both the hash node and a cached
// version of the original node. If the node already has a cached hash, returns
// that instead of recomputing.
func (h *hasher) hash(n node) (node, node) {
	if hash, _ := n.cache(); hash != nil {
		return hash, n
	}

	switch n := n.(type) {
	case *edgeNode:
		collapsed, cached := h.hashEdgeChild(n)
		hn := &hashNode{Felt: collapsed.hash(h.hashFn)}
		cached.flags.hash = hn
		return hn, cached
	case *binaryNode:
		collapsed, cached := h.hashBinaryChildren(n)
		hn := &hashNode{Felt: collapsed.hash(h.hashFn)}
		cached.flags.hash = hn
		return hn, cached
	case valueNode, hashNode:
		return n, n
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

func (h *hasher) hashEdgeChild(n *edgeNode) (collapsed, cached *edgeNode) {
	collapsed, cached = n.copy(), n.copy()

	switch n.child.(type) {
	case *edgeNode, *binaryNode:
		collapsed.child, cached.child = h.hash(n.child)
	}

	return collapsed, cached
}

func (h *hasher) hashBinaryChildren(n *binaryNode) (collapsed, cached *binaryNode) {
	collapsed, cached = n.copy(), n.copy()

	if h.parallel { // TODO(weiihann): double check this parallel strategy
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			collapsed.children[0], cached.children[0] = h.hash(n.children[0])
		}()

		go func() {
			defer wg.Done()
			collapsed.children[1], cached.children[1] = h.hash(n.children[1])
		}()

		wg.Wait()
	} else {
		collapsed.children[0], cached.children[0] = h.hash(n.children[0])
		collapsed.children[1], cached.children[1] = h.hash(n.children[1])
	}

	return collapsed, cached
}
