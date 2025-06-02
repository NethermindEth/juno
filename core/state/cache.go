package state

import "github.com/NethermindEth/juno/core/felt"

const (
	// DefaultMaxLayers is the default maximum number of layers to keep in the cache
	DefaultMaxLayers = 128
)

type diffCache struct {
	storageDiffs      map[felt.Felt]map[felt.Felt]*felt.Felt // addr -> {key -> value, ...}
	nonces            map[felt.Felt]*felt.Felt               // addr -> nonce
	deployedContracts map[felt.Felt]*felt.Felt               // addr -> class hash
}

type stateCache struct {
	diffs      map[felt.Felt]*diffCache // state root -> state diff
	links      map[felt.Felt]felt.Felt  // child -> parent
	oldestRoot felt.Felt                // root of the oldest layer in the cache
}

func newStateCache() *stateCache {
	return &stateCache{
		diffs: make(map[felt.Felt]*diffCache),
		links: make(map[felt.Felt]felt.Felt),
	}
}

// AddLayer adds a new layer to the cache and evicts old layers if necessary
func (c *stateCache) AddLayer(stateRoot, parentRoot felt.Felt, diff *diffCache) {
	if len(c.links) == 0 {
		c.oldestRoot = stateRoot
	}

	c.diffs[stateRoot] = diff
	c.links[stateRoot] = parentRoot

	c.evictOldLayers()
}

// evictOldLayers removes the oldest layers when the cache exceeds DefaultMaxLayers
func (c *stateCache) evictOldLayers() {
	for len(c.links) > DefaultMaxLayers {
		nextOldest := c.links[c.oldestRoot]

		delete(c.diffs, c.oldestRoot)
		delete(c.links, c.oldestRoot)

		c.oldestRoot = nextOldest
	}
}

func (c *stateCache) getNonce(stateRoot, addr *felt.Felt) *felt.Felt {
	diff, exists := c.diffs[*stateRoot]
	if !exists {
		if parent, ok := c.links[*stateRoot]; ok {
			return c.getNonce(&parent, addr)
		}
		return nil
	}

	if nonce, ok := diff.nonces[*addr]; ok {
		return nonce
	}

	if parent, ok := c.links[*stateRoot]; ok {
		return c.getNonce(&parent, addr)
	}

	return nil
}

func (c *stateCache) getStorageDiff(stateRoot, addr *felt.Felt) map[felt.Felt]*felt.Felt {
	diff, exists := c.diffs[*stateRoot]
	if !exists {
		if parent, ok := c.links[*stateRoot]; ok {
			return c.getStorageDiff(&parent, addr)
		}
		return nil
	}

	if storage, ok := diff.storageDiffs[*addr]; ok {
		return storage
	}

	if parent, ok := c.links[*stateRoot]; ok {
		return c.getStorageDiff(&parent, addr)
	}

	return nil
}

func (c *stateCache) getDeployedContract(stateRoot, addr *felt.Felt) *felt.Felt {
	diff, exists := c.diffs[*stateRoot]
	if !exists {
		if parent, ok := c.links[*stateRoot]; ok {
			return c.getDeployedContract(&parent, addr)
		}
		return nil
	}

	if classHash, ok := diff.deployedContracts[*addr]; ok {
		return classHash
	}

	if parent, ok := c.links[*stateRoot]; ok {
		return c.getDeployedContract(&parent, addr)
	}

	return nil
}
