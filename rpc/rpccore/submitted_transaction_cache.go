package rpccore

import (
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common/lru"
)

type SubmittedTransactionsCache struct {
	cache    lru.BasicLRU[felt.Felt, time.Time]
	entryTTL time.Duration
	mu       sync.Mutex
}

func NewSubmittedTransactionsCache(capacity int, entryTTL time.Duration) *SubmittedTransactionsCache {
	return &SubmittedTransactionsCache{
		cache:    lru.NewBasicLRU[felt.Felt, time.Time](capacity),
		entryTTL: entryTTL,
	}
}

// Adds new entry to cache. Flushes all expired entries.
func (c *SubmittedTransactionsCache) Add(txnHash felt.Felt) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.flush()
	c.cache.Add(txnHash, time.Now())
}

// Returns true if the entry exists in the cache and has not exceeded its lifespan; otherwise, returns false.
func (c *SubmittedTransactionsCache) Contains(txnHash felt.Felt) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	val, hit := c.cache.Get(txnHash)
	if !hit {
		return false
	}

	if time.Since(val) > c.entryTTL {
		c.cache.Remove(txnHash)
		return false
	}

	return true
}

// Remove deletes the entry for txnHash from the cache if it exists.
// It returns true if the entry was present and removed, or false otherwise.
func (c *SubmittedTransactionsCache) Remove(txnHash felt.Felt) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cache.Remove(txnHash)
}

// Removes all entries that have exceeded their lifespan.
func (c *SubmittedTransactionsCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.flush()
}

// Removes all entries that have exceeded their lifespan.
func (c *SubmittedTransactionsCache) flush() {
	expiredKeys := make([]felt.Felt, 0)
	for _, k := range c.cache.Keys() {
		val, ok := c.cache.Peek(k)
		if ok && time.Since(val) > c.entryTTL {
			expiredKeys = append(expiredKeys, k)
		}
	}

	for _, k := range expiredKeys {
		c.cache.Remove(k)
	}
}
