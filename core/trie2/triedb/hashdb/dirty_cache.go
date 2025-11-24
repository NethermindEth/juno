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
	size                 int
}

func newDirtyCache() *dirtyCache {
	return &dirtyCache{
		classNodes:           make(map[string]trienode.TrieNode),
		contractNodes:        make(map[string]trienode.TrieNode),
		contractStorageNodes: make(map[felt.Address]map[string]trienode.TrieNode),
	}
}

func (c *dirtyCache) putNode(
	owner *felt.Address, path *trieutils.Path, hash *felt.Felt, isClass bool, node trienode.TrieNode,
) {
	key := nodeKey(path, hash)
	keyStr := string(key)

	if isClass {
		c.classNodes[keyStr] = node
	}

	if (*felt.Felt)(owner).IsZero() {
		c.contractNodes[keyStr] = node
	} else {
		if _, ok := c.contractStorageNodes[*owner]; !ok {
			c.contractStorageNodes[*owner] = make(map[string]trienode.TrieNode)
		}
		c.contractStorageNodes[*owner][keyStr] = node
	}
}

func (c *dirtyCache) getNode(
	owner *felt.Address, path *trieutils.Path, hash *felt.Felt, isClass bool,
) (trienode.TrieNode, bool) {
	key := nodeKey(path, hash)
	keyStr := string(key)

	if isClass {
		node, ok := c.classNodes[keyStr]
		return node, ok
	}

	if (*felt.Felt)(owner).IsZero() {
		node, ok := c.contractNodes[keyStr]
		return node, ok
	}

	ownerNodes, ok := c.contractStorageNodes[*owner]
	if !ok {
		return trienode.NewLeaf(felt.Zero, nil), false
	}

	node, ok := ownerNodes[keyStr]
	return node, ok
}

func (c *dirtyCache) len() int {
	return len(c.classNodes) + len(c.contractNodes) + len(c.contractStorageNodes)
}

func (c *dirtyCache) reset() {
	c.classNodes = make(map[string]trienode.TrieNode)
	c.contractNodes = make(map[string]trienode.TrieNode)
	c.contractStorageNodes = make(map[felt.Address]map[string]trienode.TrieNode)
	c.size = 0
}
