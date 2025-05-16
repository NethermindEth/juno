package trie2

import (
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/trie2/trienode"
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
func (h *hasher) hash(n trienode.Node) (trienode.Node, trienode.Node) {
	if hash, _ := n.Cache(); hash != nil {
		return hash, n
	}

	switch n := n.(type) {
	case *trienode.EdgeNode:
		collapsed, cached := h.hashEdgeChild(n)
		hash := collapsed.Hash(h.hashFn)
		hn := (*trienode.HashNode)(&hash)
		cached.Flags.Hash = hn
		return hn, cached
	case *trienode.BinaryNode:
		collapsed, cached := h.hashBinaryChildren(n)
		hash := collapsed.Hash(h.hashFn)
		hn := (*trienode.HashNode)(&hash)
		cached.Flags.Hash = hn
		return hn, cached
	case *trienode.ValueNode, *trienode.HashNode:
		return n, n
	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}

func (h *hasher) hashEdgeChild(n *trienode.EdgeNode) (collapsed, cached *trienode.EdgeNode) {
	collapsed, cached = n.Copy(), n.Copy()

	switch n.Child.(type) {
	case *trienode.EdgeNode, *trienode.BinaryNode:
		collapsed.Child, cached.Child = h.hash(n.Child)
	}

	return collapsed, cached
}

func (h *hasher) hashBinaryChildren(n *trienode.BinaryNode) (collapsed, cached *trienode.BinaryNode) {
	collapsed, cached = n.Copy(), n.Copy()

	if h.parallel { // TODO(weiihann): double check this parallel strategy
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			if n.Left() != nil {
				collapsed.Children[0], cached.Children[0] = h.hash(n.Left())
			} else {
				collapsed.Children[0], cached.Children[0] = trienode.NilValueNode, trienode.NilValueNode
			}
		}()

		go func() {
			defer wg.Done()
			if n.Right() != nil {
				collapsed.Children[1], cached.Children[1] = h.hash(n.Right())
			} else {
				collapsed.Children[1], cached.Children[1] = trienode.NilValueNode, trienode.NilValueNode
			}
		}()

		wg.Wait()
	} else {
		if n.Left() != nil {
			collapsed.Children[0], cached.Children[0] = h.hash(n.Left())
		} else {
			collapsed.Children[0], cached.Children[0] = trienode.NilValueNode, trienode.NilValueNode
		}

		if n.Right() != nil {
			collapsed.Children[1], cached.Children[1] = h.hash(n.Right())
		} else {
			collapsed.Children[1], cached.Children[1] = trienode.NilValueNode, trienode.NilValueNode
		}
	}

	return collapsed, cached
}

// Construct trie proofs and returns the collapsed node (i.e. nodes with hash children)
// and the hashed node.
func (h *hasher) proofHash(original trienode.Node) (collapsed, hashed trienode.Node) {
	switch n := original.(type) {
	case *trienode.EdgeNode:
		en, _ := h.hashEdgeChild(n)
		hash := en.Hash(h.hashFn)
		return en, (*trienode.HashNode)(&hash)
	case *trienode.BinaryNode:
		bn, _ := h.hashBinaryChildren(n)
		hash := bn.Hash(h.hashFn)
		return bn, (*trienode.HashNode)(&hash)
	default:
		return n, n
	}
}
