package hashdb

import (
	"bytes"
	"sync"

	"github.com/ethereum/go-ethereum/common/lru"
)

// Cache is a LRU cache.
// This type is safe for concurrent use.
type LRUCache struct {
	cache  lru.BasicLRU[string, []byte]
	mu     sync.Mutex
	hits   uint64
	misses uint64
}

// NewCache creates an LRU cache.
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{cache: lru.NewBasicLRU[string, []byte](capacity)}
}

// Add adds a value to the cache. Returns true if an item was evicted to store the new item.
func (c *LRUCache) Set(key []byte, value []byte) (evicted bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cache.Add(string(key), value)
}

// Get retrieves a value from the cache. This marks the key as recently used.
func (c *LRUCache) Get(buf *bytes.Buffer, key []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	cachedValue, hit := c.cache.Get(string(key))
	if !hit {
		c.misses++
		return false
	}
	buf.Write(cachedValue)
	c.hits++
	return true
}

func (c *LRUCache) Peek(key []byte) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cache.Peek(string(key))
}

// Len returns the current number of items in the cache.
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cache.Len()
}

// Remove drops an item from the cache. Returns true if the key was present in cache.
func (c *LRUCache) Remove(key []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cache.Remove(string(key))
}

func (c *LRUCache) GetOldest() ([]byte, []byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key, value, ok := c.cache.GetOldest()
	return []byte(key), value, ok
}

func (c *LRUCache) RemoveOldest() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, _, ok := c.cache.RemoveOldest()
	return ok
}

func (c *LRUCache) Hits() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.hits
}

func (c *LRUCache) HitRate() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hits+c.misses == 0 {
		return 0
	}

	return float64(c.hits) / float64(c.hits+c.misses)
}
