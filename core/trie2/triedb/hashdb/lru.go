package hashdb

import (
	"bytes"

	"github.com/ethereum/go-ethereum/common/lru"
)

type LRUCache struct {
	cache  *lru.Cache[string, []byte]
	hits   uint64
	misses uint64
}

func NewLRUCache(size int) *LRUCache {
	return &LRUCache{
		cache: lru.NewCache[string, []byte](size),
	}
}

func (c *LRUCache) Get(buf *bytes.Buffer, key []byte) bool {
	stringKey := string(key)
	value, hit := c.cache.Get(stringKey)
	if !hit {
		c.misses++
		return false
	}
	buf.Write(value)
	c.hits++
	return true
}

func (c *LRUCache) Set(key, value []byte) {
	stringKey := string(key)
	c.cache.Add(stringKey, value)
}

func (c *LRUCache) Delete(key []byte) {
	stringKey := string(key)
	c.cache.Remove(stringKey)
}

func (c *LRUCache) Hits() uint64 {
	return c.hits
}

func (c *LRUCache) HitRate() float64 {
	if c.hits+c.misses == 0 {
		return 0
	}

	return float64(c.hits) / float64(c.hits+c.misses)
}
