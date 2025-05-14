package hashdb

import (
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

type DirtyCache struct {
	classNodes           map[string][]byte
	contractNodes        map[string][]byte
	contractStorageNodes map[felt.Felt]map[string][]byte
	mu                   sync.Mutex
}

func NewDirtyCache(capacity int) *DirtyCache {
	return &DirtyCache{
		classNodes:           make(map[string][]byte),
		contractNodes:        make(map[string][]byte),
		contractStorageNodes: make(map[felt.Felt]map[string][]byte),
	}
}

func (c *DirtyCache) Set(key, value []byte, trieType trieutils.TrieType, owner felt.Felt) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyStr := string(key)
	switch trieType {
	case trieutils.Class:
		c.classNodes[keyStr] = value
	case trieutils.Contract:
		c.contractNodes[keyStr] = value
	case trieutils.ContractStorage:
		if _, ok := c.contractStorageNodes[owner]; !ok {
			c.contractStorageNodes[owner] = make(map[string][]byte)
		}
		c.contractStorageNodes[owner][keyStr] = value
	}
}

func (c *DirtyCache) Get(key []byte, trieType trieutils.TrieType, owner felt.Felt) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyStr := string(key)
	switch trieType {
	case trieutils.Class:
		cachedValue, hit := c.classNodes[keyStr]
		if !hit {
			return nil, false
		}
		return cachedValue, true
	case trieutils.Contract:
		cachedValue, hit := c.contractNodes[keyStr]
		if !hit {
			return nil, false
		}
		return cachedValue, true
	case trieutils.ContractStorage:
		ownerNodes, ok := c.contractStorageNodes[owner]
		if !ok {
			return nil, false
		}
		cachedValue, hit := ownerNodes[keyStr]
		if !hit {
			return nil, false
		}
		return cachedValue, true
	}
	panic("unknown trie type")
}

func (c *DirtyCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.classNodes) + len(c.contractNodes) + len(c.contractStorageNodes)
}

func (c *DirtyCache) Remove(key []byte, trieType trieutils.TrieType, owner felt.Felt) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyStr := string(key)
	switch trieType {
	case trieutils.Class:
		if _, ok := c.classNodes[keyStr]; ok {
			delete(c.classNodes, keyStr)
			return nil
		}
		return fmt.Errorf("key %x not found", key)
	case trieutils.Contract:
		if _, ok := c.contractNodes[keyStr]; ok {
			delete(c.contractNodes, keyStr)
			return nil
		}
		return fmt.Errorf("key %x not found", key)
	case trieutils.ContractStorage:
		ownerNodes, ok := c.contractStorageNodes[owner]
		if !ok {
			return fmt.Errorf("owner %x not found", owner.Bytes())
		}
		if _, ok := ownerNodes[keyStr]; ok {
			delete(ownerNodes, keyStr)
			if len(ownerNodes) == 0 {
				delete(c.contractStorageNodes, owner)
			}
			return nil
		}
		return fmt.Errorf("key %x not found", key)
	}
	return fmt.Errorf("unknown trie type")
}
