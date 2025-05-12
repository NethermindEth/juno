package hashdb

import (
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

// Cache is a LRU cache.
// This type is safe for concurrent use.
type DirtyCache struct {
	classNodes           map[string][]byte
	contractNodes        map[string][]byte
	contractStorageNodes map[felt.Felt]map[string][]byte
	mu                   sync.Mutex
	hits                 uint64
	misses               uint64
}

// NewCache creates an LRU cache.
func NewDirtyCache(capacity int) *DirtyCache {
	return &DirtyCache{
		classNodes:           make(map[string][]byte),
		contractNodes:        make(map[string][]byte),
		contractStorageNodes: make(map[felt.Felt]map[string][]byte),
	}
}

// Add adds a value to the cache. Returns true if an item was evicted to store the new item.
func (c *DirtyCache) Set(key []byte, value []byte, trieType trieutils.TrieType, owner felt.Felt) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch trieType {
	case trieutils.Class:
		c.classNodes[string(key)] = value
	case trieutils.Contract:
		c.contractNodes[string(key)] = value
	case trieutils.ContractStorage:
		if _, ok := c.contractStorageNodes[owner]; !ok {
			c.contractStorageNodes[owner] = make(map[string][]byte)
		}
		c.contractStorageNodes[owner][string(key)] = value
	}
}

// Get retrieves a value from the cache. This marks the key as recently used.
func (c *DirtyCache) Get(key []byte, trieType trieutils.TrieType, owner felt.Felt) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch trieType {
	case trieutils.Class:
		cachedValue, hit := c.classNodes[string(key)]
		if !hit {
			c.misses++
			return nil, false
		}
		c.hits++
		return cachedValue, true
	case trieutils.Contract:
		cachedValue, hit := c.contractNodes[string(key)]
		if !hit {
			c.misses++
			return nil, false
		}
		c.hits++
		return cachedValue, true
	case trieutils.ContractStorage:
		ownerNodes, ok := c.contractStorageNodes[owner]
		if !ok {
			c.misses++
			return nil, false
		}
		cachedValue, hit := ownerNodes[string(key)]
		if !hit {
			c.misses++
			return nil, false
		}
		c.hits++
		return cachedValue, true
	}
	panic("unknown trie type")
}

// Len returns the current number of items in the cache.
func (c *DirtyCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.classNodes) + len(c.contractNodes) + len(c.contractStorageNodes)
}

// Remove drops an item from the cache. Returns true if the key was present in cache.
func (c *DirtyCache) Remove(key []byte, trieType trieutils.TrieType, owner felt.Felt) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch trieType {
	case trieutils.Class:
		delete(c.classNodes, string(key))
	case trieutils.Contract:
		delete(c.contractNodes, string(key))
	case trieutils.ContractStorage:
		ownerNodes, ok := c.contractStorageNodes[owner]
		if !ok {
			return fmt.Errorf("owner %s not found", owner)
		}
		delete(ownerNodes, string(key))
		if len(ownerNodes) == 0 {
			delete(c.contractStorageNodes, owner)
		}
	}
	return nil
}

func (c *DirtyCache) Hits() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.hits
}

func (c *DirtyCache) HitRate() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hits+c.misses == 0 {
		return 0
	}

	return float64(c.hits) / float64(c.hits+c.misses)
}
