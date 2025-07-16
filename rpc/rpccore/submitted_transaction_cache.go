package rpccore

import (
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common/lru"
)

// SubmittedTransactionsCache is an in-memory cache that tracks transaction hashes
// recently submitted to the mempool, but not yet included in a block.
//
// This allows the system to quickly determine if a transaction was recently received,
// improving accuracy and user experience for transaction status queries during the
// period between mempool acceptance and block inclusion.
//
// Entries automatically expire after a configurable TTL.
type SubmittedTransactionsCache struct {
	cache    lru.BasicLRU[felt.Felt, time.Time] // Requires storing in map and list, and maintaining both
	entryTTL time.Duration
	mu       sync.Mutex // Locks the entire map. Even on reads!
}

func NewSubmittedTransactionsCache(capacity int, entryTTL time.Duration) *SubmittedTransactionsCache {
	return &SubmittedTransactionsCache{
		cache:    lru.NewBasicLRU[felt.Felt, time.Time](capacity),
		entryTTL: entryTTL,
	}
}

// Adds new entry to cache. Flushes all expired entries.
func (c *SubmittedTransactionsCache) Add(txnHash *felt.Felt) {
	c.mu.Lock() // Locks entire map
	defer c.mu.Unlock()

	c.flush()                         // O(n)
	c.cache.Add(*txnHash, time.Now()) // O(1). Add to map. Add to List. Push to front.
}

// Returns true if the entry exists in the cache and has not exceeded its lifespan; otherwise, returns false.
func (c *SubmittedTransactionsCache) Contains(txnHash *felt.Felt) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	val, hit := c.cache.Get(*txnHash) // O(1). Get from map. Push to front.
	if !hit {
		return false
	}

	if time.Since(val) > c.entryTTL {
		c.cache.Remove(*txnHash)
		return false
	}

	return true
}

// Remove deletes the entry for txnHash from the cache if it exists.
// It returns true if the entry was present and removed, or false otherwise.
func (c *SubmittedTransactionsCache) Remove(txnHash *felt.Felt) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.cache.Remove(*txnHash) // O(1). Remove from map and list.
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
	for _, k := range c.cache.Keys() { // O(n)
		val, ok := c.cache.Peek(k) // O(1)
		if ok && time.Since(val) > c.entryTTL {
			expiredKeys = append(expiredKeys, k) // Potentially expensive resize
		}
	}

	for _, k := range expiredKeys { // O(n)..
		c.cache.Remove(k)
	}
}
