package state

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

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

func (c *stateCache) PushLayer(stateRoot, parentRoot *felt.Felt, diff *diffCache) {
	if len(c.links) == 0 {
		c.oldestRoot = *stateRoot
	}

	c.diffs[*stateRoot] = diff
	c.links[*stateRoot] = *parentRoot

	for len(c.links) > DefaultMaxLayers {
		// Find the child of the current oldest root
		var nextOldest felt.Felt
		for child, parent := range c.links {
			if parent == c.oldestRoot {
				nextOldest = child
				break
			}
		}

		delete(c.diffs, c.oldestRoot)
		delete(c.links, c.oldestRoot)

		c.oldestRoot = nextOldest
	}
}

func (c *stateCache) PopLayer(stateRoot *felt.Felt) error {
	if _, exists := c.diffs[*stateRoot]; !exists {
		return fmt.Errorf("layer with state root %v not found", stateRoot)
	}

	for _, parent := range c.links {
		if parent.Equal(stateRoot) {
			return fmt.Errorf("cannot pop layer %v: it is not the newest layer", stateRoot)
		}
	}

	delete(c.diffs, *stateRoot)
	delete(c.links, *stateRoot)

	return nil
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

func (c *stateCache) getStorageDiff(stateRoot, addr, key *felt.Felt) *felt.Felt {
	diff, exists := c.diffs[*stateRoot]
	if !exists {
		if parent, ok := c.links[*stateRoot]; ok {
			return c.getStorageDiff(&parent, addr, key)
		}
		return nil
	}

	if storage, ok := diff.storageDiffs[*addr]; ok {
		if value, ok := storage[*key]; ok {
			return value
		}
		return nil
	}

	if parent, ok := c.links[*stateRoot]; ok {
		return c.getStorageDiff(&parent, addr, key)
	}

	return nil
}

func (c *stateCache) getReplacedClass(stateRoot, addr *felt.Felt) *felt.Felt {
	diff, exists := c.diffs[*stateRoot]
	if !exists {
		if parent, ok := c.links[*stateRoot]; ok {
			return c.getReplacedClass(&parent, addr)
		}
		return nil
	}

	if classHash, ok := diff.deployedContracts[*addr]; ok {
		return classHash
	}

	if parent, ok := c.links[*stateRoot]; ok {
		return c.getReplacedClass(&parent, addr)
	}

	return nil
}
