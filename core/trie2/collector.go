package trie2

import (
	"fmt"
	"sync"

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
func (c *collector) Collect(n Node, parallel bool) *HashNode {
	return c.collect(new(Path), n, parallel).(*HashNode)
}

func (c *collector) collect(path *Path, n Node, parallel bool) Node {
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
		collapsed.Child = c.collect(new(Path).Append(path, cn.Path), cn.Child, parallel)
		return c.store(path, collapsed)
	case *BinaryNode:
		collapsed := cn.copy()
		collapsed.Children = c.collectChildren(path, cn, parallel)
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
func (c *collector) collectChildren(path *Path, n *BinaryNode, parallel bool) [2]Node {
	children := [2]Node{}

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

	for i := range 2 {
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
func (c *collector) store(path *Path, n Node) Node {
	hash, _ := n.cache()

	blob := nodeToBytes(n)
	if hash == nil { // this is a value node
		c.nodes.Add(*path, trienode.NewLeaf(blob))
		return n
	}

	c.nodes.Add(*path, trienode.NewNonLeaf(hash.Felt, blob))
	return hash
}
