package hashdb

import (
	"sync"

	"github.com/VictoriaMetrics/fastcache"
)

type FastCache struct {
	cache *fastcache.Cache
	mu    sync.RWMutex
}

func NewFastCache(size int) *FastCache {
	if size <= 0 {
		return nil
	}
	return &FastCache{
		cache: fastcache.New(size),
		mu:    sync.RWMutex{},
	}
}

func (c *FastCache) Get(key []byte) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cache == nil {
		return nil, false
	}

	value := c.cache.Get(nil, key)
	if value == nil {
		return nil, false
	}

	return value, true
}

func (c *FastCache) Set(key, value []byte) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		panic("cache is nil")
	}

	c.cache.Set(key, value)
}

func (c *FastCache) Remove(key []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		return false
	}

	c.cache.Del(key)
	return true
}
