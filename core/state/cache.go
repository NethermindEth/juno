package state

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

// DefaultMaxLayers is the default maximum number of layers to keep in the cache
const DefaultMaxLayers = 128

type diffCache struct {
	storageDiffs      map[felt.Felt]map[felt.Felt]*felt.Felt // addr -> {key -> value, ...}
	nonces            map[felt.Felt]*felt.Felt               // addr -> nonce
	deployedContracts map[felt.Felt]*felt.Felt               // addr -> class hash
}

type cacheNode struct {
	root   felt.Felt
	parent *cacheNode
	child  *cacheNode
	diff   diffCache
}

// stateCache is a clean, in-memory cache of the state, where the stateDiffs are stored. After every state update,
// a new layer - a stateDiff associated with the state root is added to the cache. The state cache is implemented as a linked list.

type stateCache struct {
	rootMap    map[felt.Felt]*cacheNode
	oldestNode *cacheNode
	newestNode *cacheNode
}

func newStateCache() stateCache {
	return stateCache{
		rootMap: make(map[felt.Felt]*cacheNode, DefaultMaxLayers),
	}
}

// PushLayer adds a new stateDiff associated with the state root to the cache.
// If the cache is full, the oldest layer is removed. By default, the cache is limited to 128 layers.
// If the state root is the same as the parent root, the layer is not added to the cache.
func (c *stateCache) PushLayer(stateRoot, parentRoot *felt.Felt, diff *diffCache) {
	if stateRoot.Equal(parentRoot) {
		return
	}

	// Create new node
	node := &cacheNode{
		root:   *stateRoot,
		diff:   *diff,
		parent: c.newestNode,
	}
	if c.newestNode != nil {
		c.newestNode.child = node
	}
	c.newestNode = node
	if c.oldestNode == nil {
		c.oldestNode = node
	}
	c.rootMap[*stateRoot] = node

	// Evict if over capacity
	if len(c.rootMap) > DefaultMaxLayers {
		evict := c.oldestNode
		c.oldestNode = evict.child
		if c.oldestNode != nil {
			c.oldestNode.parent = nil
		}
		delete(c.rootMap, evict.root)
	}
}

// PopLayer removes the layer associated with the state root from the cache.
// It returns an error if the layer is not found or if it is not the newest layer.
// If there was no change in the state, the layer is not cached, hence it is not removed.
func (c *stateCache) PopLayer(stateRoot, parentRoot *felt.Felt) error {
	if stateRoot.Equal(parentRoot) {
		return nil
	}

	node, ok := c.rootMap[*stateRoot]
	if !ok {
		// There should be no error when layer is not found in the cache.
		// The layer might not be cached (i. e. after node shutdown).
		return nil
	}

	if node.child != nil {
		return fmt.Errorf("cannot pop layer %v: it is not the newest layer", stateRoot)
	}

	// Remove node
	if node.parent != nil {
		node.parent.child = nil
		c.newestNode = node.parent
	} else {
		c.newestNode = nil
		c.oldestNode = nil
	}
	delete(c.rootMap, *stateRoot)

	return nil
}

func (c *stateCache) getNonce(stateRoot, addr *felt.Felt) *felt.Felt {
	node := c.rootMap[*stateRoot]
	for node != nil {
		if nonce, ok := node.diff.nonces[*addr]; ok {
			return nonce
		}
		node = node.parent
	}
	return nil
}

func (c *stateCache) getStorageDiff(stateRoot, addr, key *felt.Felt) *felt.Felt {
	node := c.rootMap[*stateRoot]
	for node != nil {
		if storage, ok := node.diff.storageDiffs[*addr]; ok {
			if val, ok := storage[*key]; ok {
				return val
			}
		}
		node = node.parent
	}
	return nil
}

func (c *stateCache) getReplacedClass(stateRoot, addr *felt.Felt) *felt.Felt {
	node := c.rootMap[*stateRoot]
	for node != nil {
		if classHash, ok := node.diff.deployedContracts[*addr]; ok {
			return classHash
		}
		node = node.parent
	}
	return nil
}
