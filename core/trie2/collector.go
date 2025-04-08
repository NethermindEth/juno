package trie2

import (
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
)

const (
	parallelThreshold = 8 // TODO: arbitrary number, configure this based on monitoring
)

// Used as a tool to collect all dirty nodes in a NodeSet
type collector struct {
	nodes *trienode.NodeSet
}

func newCollector(nodes *trienode.NodeSet) *collector {
	return &collector{nodes: nodes}
}

// Collects the nodes in the node set and collapses a node into a hash node
func (c *collector) Collect(n Node, parallel bool) *HashNode {
	return c.collect(new(Path), n, parallel, 0).(*HashNode)
}

// Collects all dirty nodes in the trie and converts them to hash nodes.
// Traverses the trie recursively, processing each node type:
// - EdgeNode: Collects its child before processing the edge node itself
// - BinaryNode: Collects both children (potentially in parallel) before processing the node
// - HashNode: Already processed, returns as is
// - ValueNode: Stores as a leaf in the node set
// Returns a HashNode representing the processed node.
func (c *collector) collect(path *Path, n Node, parallel bool, depth int) Node {
	// This path has not been modified, just return the cache
	hash, dirty := n.cache()
	if hash != nil && !dirty {
		return hash
	}

	// Collect children and then parent
	switch cn := n.(type) {
	case *EdgeNode:
		collapsed := cn.copy()

		// If the child is a binary node, recurse into it.
		// Otherwise, it can only be a HashNode or ValueNode.
		// Combination of edge (parent) + edge (child) is not possible.
		collapsed.Child = c.collect(new(Path).Append(path, cn.Path), cn.Child, parallel, depth+1)
		return c.store(path, collapsed)
	case *BinaryNode:
		collapsed := cn.copy()
		collapsed.Children = c.collectChildren(path, cn, parallel, depth+1)
		return c.store(path, collapsed)
	case *HashNode:
		return cn
	case *ValueNode: // each leaf node is stored as a single entry in the node set
		return c.store(path, cn)
	default:
		panic(fmt.Sprintf("unknown node type: %T", cn))
	}
}

// Collects the children of a binary node, may apply parallel processing if configured
func (c *collector) collectChildren(path *Path, n *BinaryNode, parallel bool, depth int) [2]Node {
	children := [2]Node{}

	var mu sync.Mutex

	// Helper function to process a single child
	processChild := func(i int) {
		child := n.Children[i]
		// Return early if it's already a hash node
		if hn, ok := child.(*HashNode); ok {
			children[i] = hn
			return
		}

		// Create child path
		childPath := new(Path).AppendBit(path, uint8(i))

		if !parallel {
			children[i] = c.collect(childPath, child, parallel, depth)
			return
		}

		// Parallel processing
		childSet := trienode.NewNodeSet(c.nodes.Owner)
		childCollector := newCollector(childSet)
		children[i] = childCollector.collect(childPath, child, depth < parallelThreshold, depth)

		// Merge the child set into the parent set
		// Must be done under the mutex because node set is not thread safe
		mu.Lock()
		c.nodes.MergeSet(childSet) //nolint:errcheck // guaranteed to succeed because same owner
		mu.Unlock()
	}

	if !parallel {
		// Sequential processing
		processChild(0)
		processChild(1)
		return children
	}

	// Parallel processing
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			processChild(idx)
		}(i)
	}
	wg.Wait()

	return children
}

// Stores the node in the node set and returns the hash node
func (c *collector) store(path *Path, n Node) Node {
	hash, _ := n.cache()

	blob := nodeToBytes(n)
	if hash == nil { // this is a value node
		var h felt.Felt
		h.SetBytes(blob)
		c.nodes.Add(*path, trienode.NewLeaf(h, blob))
		return n
	}

	c.nodes.Add(*path, trienode.NewNonLeaf(hash.Felt, blob))
	return hash
}
