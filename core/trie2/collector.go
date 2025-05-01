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
func (c *collector) Collect(n trienode.Node, parallel bool) *trienode.HashNode {
	return c.collect(new(Path), n, parallel, 0).(*trienode.HashNode)
}

// Collects all dirty nodes in the trie and converts them to hash nodes.
// Traverses the trie recursively, processing each node type:
// - EdgeNode: Collects its child before processing the edge node itself
// - BinaryNode: Collects both children (potentially in parallel) before processing the node
// - trienode.HashNode: Already processed, returns as is
// - ValueNode: Stores as a leaf in the node set
// Returns a trienode.HashNode representing the processed node.
func (c *collector) collect(path *Path, n trienode.Node, parallel bool, depth int) trienode.Node {
	// This path has not been modified, just return the cache
	hash, dirty := n.Cache()
	if hash != nil && !dirty {
		return hash
	}

	// Collect children and then parent
	switch cn := n.(type) {
	case *trienode.EdgeNode:
		collapsed := cn.Copy()

		// If the child is a binary node, recurse into it.
		// Otherwise, it can only be a trienode.HashNode or ValueNode.
		// Combination of edge (parent) + edge (child) is not possible.
		collapsed.Child = c.collect(new(Path).Append(path, cn.Path), cn.Child, parallel, depth+1)
		return c.store(path, collapsed)
	case *trienode.BinaryNode:
		collapsed := cn.Copy()
		collapsed.Children = c.collectChildren(path, cn, parallel, depth+1)
		return c.store(path, collapsed)
	case *trienode.HashNode:
		return cn
	case *trienode.ValueNode: // each leaf node is stored as a single entry in the node set
		return c.store(path, cn)
	default:
		panic(fmt.Sprintf("unknown node type: %T", cn))
	}
}

// Collects the children of a binary node, may apply parallel processing if configured
func (c *collector) collectChildren(path *Path, n *trienode.BinaryNode, parallel bool, depth int) [2]trienode.Node {
	children := [2]trienode.Node{}

	var mu sync.Mutex

	// Helper function to process a single child
	processChild := func(i int) {
		child := n.Children[i]
		// Return early if it's already a hash node
		if hn, ok := child.(*trienode.HashNode); ok {
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
func (c *collector) store(path *Path, n trienode.Node) trienode.Node {
	hash, _ := n.Cache()

	blob := trienode.EncodeNode(n)
	if hash == nil { // this is a value node
		var h felt.Felt
		h.SetBytes(blob)
		c.nodes.Add(*path, trienode.NewLeaf(h, blob))
		return n
	}

	c.nodes.Add(*path, trienode.NewNonLeaf(hash.Felt, blob))
	return hash
}
