package lru

import (
	"hash/maphash"
	"sync"
)

type cacheNode struct {
	key   uint64
	value []byte
	next  *cacheNode
	prev  *cacheNode
}

// Cache is a cache with the least-recently-used policy.
type Cache struct {
	*sync.RWMutex

	// hashMap contaiins the current cache contents that can be accessed by the key
	hashMap map[uint64]*cacheNode
	// front is a reference to the first node in the cache queue
	front *cacheNode
	// back is a reference to the last node in the cache queue
	back *cacheNode
	// len is the current amount of items in the cache
	len int
	// cap is the max amount of items that can be stored in the cache
	cap int
	// hash is used to hash the key
	hash maphash.Hash
}

// NewCache creates a new Cache instance with the given cap. If cap == 0, nil is
// returned.
func NewCache(cap int) *Cache {
	if cap < 1 {
		return nil
	}
	return &Cache{
		RWMutex: new(sync.RWMutex),
		hashMap: make(map[uint64]*cacheNode, cap),
		cap:     cap,
		hash:    maphash.Hash{},
	}
}

// Put adds a new key-value pair to the cache. If the cache is full then
// the least-recently-used key-value pair is removed. If the key already exist
// in the cache then the node will be replaced and put at the front of the
// cache.
func (c *Cache) Put(k []byte, v []byte) {
	c.Lock()
	defer c.Unlock()

	key := c.key(k)
	newN := &cacheNode{key: key, value: v}

	if c.len == 0 {
		c.hashMap[key] = newN
		c.front = newN
		c.back = newN
		c.len++
		return
	}

	node, ok := c.hashMap[key]

	// key doesn't already exist in cache so add the node to the front and
	// delete the last node if cache is full
	if !ok {
		c.hashMap[key] = newN
		c.front.prev = newN
		newN.next = c.front
		c.front = newN
		c.len++

		if c.len > c.cap {
			delete(c.hashMap, c.back.key)
			c.back = c.back.prev
			c.back.next.prev = nil
			c.back.next = nil
			c.len--
		}
		return
	}

	// key already exists in the cache so replace the map reference, delete the
	// old node from the cache and add the new node to the front of the cache.
	c.hashMap[key] = newN
	if node == c.front {
		newN.next = c.front.next
		c.front.next.prev = newN
		c.front = newN
		node.next = nil
		return
	} else if node == c.back {
		c.back = node.prev
		node.prev = nil
	} else {
		node.prev.next = node.next
		node.next.prev = node.prev
		node.next = nil
		node.prev = nil
	}
	c.front.prev = newN
	newN.next = c.front
	c.front = newN
}

// Get returns the value for the given key. If the key is not found then
// returns nil.
func (c *Cache) Get(k []byte) []byte {
	c.Lock()
	defer c.Unlock()

	key := c.key(k)
	if node, ok := c.hashMap[key]; ok {
		if node != c.front {
			// Move the node to the front of the queue
			node.prev.next = node.next
			if node == c.back {
				c.back = node.prev
			} else {
				node.next.prev = node.prev
			}
			c.front.prev = node
			node.next = c.front
			node.prev = nil
			c.front = node
		}
		v := make([]byte, len(node.value))
		copy(v, node.value)
		return v
	}
	return nil
}

// Front returns a copy of the value at front of the cache
func (c Cache) Front() []byte {
	c.RLock()
	defer c.RUnlock()

	if c.front != nil {
		v := make([]byte, len(c.front.value))
		copy(v, c.front.value)
		return v
	}
	return nil
}

// Back returns a copy of value at the back of the cache
func (c Cache) Back() []byte {
	c.RLock()
	defer c.RUnlock()

	if c.back != nil {
		v := make([]byte, len(c.back.value))
		copy(v, c.back.value)
		return v
	}
	return nil
}

// Len returns the current amount of items in the cache.
func (c *Cache) Len() int {
	c.RLock()
	defer c.RUnlock()
	return c.len
}

// Cap returns the max amount of items that can be stored in the cache.
func (c *Cache) Cap() int {
	c.RLock()
	defer c.RUnlock()
	return c.cap
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.Lock()
	defer c.Unlock()

	c.hashMap = make(map[uint64]*cacheNode, c.cap)
	c.front = nil
	c.back = nil
	c.len = 0
}

func (c *Cache) key(v []byte) uint64 {
	c.hash.Reset()
	c.hash.Write(v)
	return c.hash.Sum64()
}
