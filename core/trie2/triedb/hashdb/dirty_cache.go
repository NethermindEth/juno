package hashdb

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

type dirtyCache struct {
	classNodes           map[string]trienode.TrieNode
	contractNodes        map[string]trienode.TrieNode
	contractStorageNodes map[felt.Address]map[string]trienode.TrieNode
}

func newDirtyCache() *dirtyCache {
	return &dirtyCache{
		classNodes:           make(map[string]trienode.TrieNode),
		contractNodes:        make(map[string]trienode.TrieNode),
		contractStorageNodes: make(map[felt.Address]map[string]trienode.TrieNode),
	}
}

func (c *dirtyCache) putNode(
	owner *felt.Address,
	path *trieutils.Path,
	hash *felt.Hash,
	isClass bool,
	node trienode.TrieNode,
) {
	key := nodeKey(path, hash)
	keyStr := string(key)

	if isClass {
		c.classNodes[keyStr] = node
		return
	}

	if felt.IsZero(owner) {
		c.contractNodes[keyStr] = node
	} else {
		if _, ok := c.contractStorageNodes[*owner]; !ok {
			c.contractStorageNodes[*owner] = make(map[string]trienode.TrieNode)
		}
		c.contractStorageNodes[*owner][keyStr] = node
	}
}

func (c *dirtyCache) getNode(
	owner *felt.Address,
	path *trieutils.Path,
	hash *felt.Hash,
	isClass bool,
) (trienode.TrieNode, bool) {
	key := nodeKey(path, hash)

	if isClass {
		node, ok := c.classNodes[string(key)]
		return node, ok
	}

	if felt.IsZero(owner) {
		node, ok := c.contractNodes[string(key)]
		return node, ok
	}

	ownerNodes, ok := c.contractStorageNodes[*owner]
	if !ok {
		return trienode.NewLeaf(felt.Zero, nil), false
	}

	node, ok := ownerNodes[string(key)]
	return node, ok
}

func (c *dirtyCache) len() int {
	n := len(c.classNodes) + len(c.contractNodes)
	for _, nodes := range c.contractStorageNodes {
		n += len(nodes)
	}
	return n
}

func (c *dirtyCache) reset() {
	c.classNodes = make(map[string]trienode.TrieNode)
	c.contractNodes = make(map[string]trienode.TrieNode)
	c.contractStorageNodes = make(map[felt.Address]map[string]trienode.TrieNode)
}
