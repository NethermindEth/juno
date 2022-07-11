package cache

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

// LRUCache is a cache with the least-recently-used policy.
type LRUCache struct {
	// hashMap contaiins the current cache contents that can be accessed by the key
	hashMap map[uint64]*cacheNode
	// start is a reference to the first node in the cache queue
	start *cacheNode
	// end is a reference to the last node in the cache queue
	end *cacheNode
	// count is the current ammount of items in the cache
	count int
	// capacity is the max ammount of items that can be stored in the cache
	capacity int
	// hash is used to hash the key
	hash maphash.Hash
	// lock is used to protect the cache from concurrent access
	lock sync.Mutex
}

// NewLRUCache creates a new LRUCache instance with the given capacity.
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		hashMap:  make(map[uint64]*cacheNode, capacity),
		capacity: capacity,
		hash:     maphash.Hash{},
	}
}

// Put adds a new key-value pair to the cache. If the cache is full then
// the least-recently-used key-value pair is removed.
func (c *LRUCache) Put(k []byte, v []byte) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Build the cache key and node
	key := c.key(k)
	node := &cacheNode{key: key, value: v}

	// Cache is empty
	if c.count == 0 {
		c.hashMap[key] = node
		c.start = node
		c.end = node
		c.count = 1
		return
	}

	// Put the new node at the start of the queue
	c.hashMap[key] = node
	c.start.prev = node
	node.next = c.start
	c.start = node
	c.count++

	// Remove the last node if the cache is full
	if c.count > c.capacity {
		delete(c.hashMap, c.end.key)
		c.end = c.end.prev
		c.end.next.prev = nil
		c.end.next = nil
		c.count--
	}
}

// Get returns the value for the given key. If the key is not found then
// returns nil.
func (c *LRUCache) Get(k []byte) []byte {
	c.lock.Lock()
	defer c.lock.Unlock()

	key := c.key(k)
	if node, ok := c.hashMap[key]; ok {
		if node != c.start {
			// Move the node to the start of the queue
			node.prev.next = node.next
			if node == c.end {
				c.end = node.prev
			} else {
				node.next.prev = node.prev
			}
			c.start.prev = node
			node.next = c.start
			node.prev = nil
			c.start = node
		}
		return node.value
	}
	return nil
}

// Len returns the current ammount of items in the cache.
func (c *LRUCache) Len() int {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.count
}

// Cap returns the max ammount of items that can be stored in the cache.
func (c *LRUCache) Cap() int {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.capacity
}

// Clear removes all items from the cache.
func (c *LRUCache) Clear() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.hashMap = make(map[uint64]*cacheNode, c.capacity)
	c.start = nil
	c.end = nil
	c.count = 0
}

func (c *LRUCache) key(v []byte) uint64 {
	c.hash.Reset()
	c.hash.Write(v)
	return c.hash.Sum64()
}
