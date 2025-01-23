package trie2

import (
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
)

// Used as a tool to collect all dirty nodes in a NodeSet
type collector struct {
	nodes *trienode.NodeSet
}

func newCollector(nodes *trienode.NodeSet) *collector {
	return &collector{nodes: nodes}
}

// Collects the nodes in the node set and collapses a node into a hash node
func (c *collector) Collect(n node, parallel bool) *hashNode {
	return c.collect(new(Path), n, parallel).(*hashNode)
}

func (c *collector) collect(path *Path, n node, parallel bool) node {
	// This path has not been modified, just return the cache
	hash, dirty := n.cache()
	if hash != nil && !dirty {
		return hash
	}

	// Collect children and then parent
	switch cn := n.(type) {
	case *edgeNode:
		collapsed := cn.copy()

		// If the child is a binary node, recurse into it.
		// Otherwise, it can only be a hashNode or valueNode.
		// Combination of edge (parent) + edge (child) is not possible.
		collapsed.child = c.collect(new(Path).Append(path, cn.path), cn.child, parallel)
		return c.store(path, collapsed)
	case *binaryNode:
		collapsed := cn.copy()
		collapsed.children = c.collectChildren(path, cn, parallel)
		return c.store(path, collapsed)
	case *hashNode:
		return cn
	case *valueNode: // each leaf node is stored as a single entry in the node set
		return c.store(path, cn)
	default:
		panic(fmt.Sprintf("unknown node type: %T", cn))
	}
}

// Collects the children of a binary node, may apply parallel processing if configured
func (c *collector) collectChildren(path *Path, n *binaryNode, parallel bool) [2]node {
	children := [2]node{}

	// Helper function to process a single child
	processChild := func(i int) {
		child := n.children[i]
		// Return early if it's already a hash node
		if hn, ok := child.(*hashNode); ok {
			children[i] = hn
			return
		}

		// Create child path
		childPath := new(Path).AppendBit(path, uint8(i))

		if !parallel {
			children[i] = c.collect(childPath, child, parallel)
			return
		}

		// Parallel processing
		childSet := trienode.NewNodeSet(c.nodes.Owner)
		childCollector := newCollector(childSet)
		children[i] = childCollector.collect(childPath, child, parallel)
		c.nodes.MergeSet(childSet) //nolint:errcheck // guaranteed to succeed because same owner
	}

	if !parallel {
		// Sequential processing
		processChild(0)
		processChild(1)
		return children
	}

	// Parallel processing
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mu.Lock()
			processChild(idx)
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	return children
}

// Stores the node in the node set and returns the hash node
func (c *collector) store(path *Path, n node) node {
	hash, _ := n.cache()

	blob := nodeToBytes(n)
	if hash == nil { // this is a value node
		c.nodes.Add(*path, trienode.NewNode(felt.Felt{}, blob))
		return n
	}

	c.nodes.Add(*path, trienode.NewNode(hash.Felt, blob))
	return hash
}
