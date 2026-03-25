package propeller

import (
	"sync"
	"time"
)

// TimeCache is a set data structure where entries automatically expire after a
// configured TTL. It is used to remember which messages have been finalised so
// we can reject late-arriving shards without keeping state forever.
//
// The cache is safe for concurrent access. Expired entries are lazily removed:
// Contains() ignores expired entries, and Cleanup() bulk-removes them. This
// amortised approach avoids the overhead of per-entry timers.
type TimeCache[K comparable] struct {
	mu      sync.Mutex
	entries map[K]time.Time
	ttl     time.Duration
	// nowFn is injectable for testing. In production it is time.Now.
	nowFn func() time.Time
}

// NewTimeCache creates a cache where entries expire after the given TTL.
func NewTimeCache[K comparable](ttl time.Duration) *TimeCache[K] {
	return &TimeCache[K]{
		entries: make(map[K]time.Time),
		ttl:     ttl,
		nowFn:   time.Now,
	}
}

// Add inserts a key into the cache with an expiry of now + TTL.
// If the key already exists, its expiry is refreshed.
func (c *TimeCache[K]) Add(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = c.nowFn().Add(c.ttl)
}

// Contains returns true if the key is present and has not expired.
// Expired keys are treated as absent but not removed -- call Cleanup()
// periodically to reclaim memory.
func (c *TimeCache[K]) Contains(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	expiry, ok := c.entries[key]
	if !ok {
		return false
	}
	return c.nowFn().Before(expiry)
}

// Cleanup removes all expired entries from the cache. Call this periodically
// (e.g., every N operations or on a timer) to prevent unbounded growth from
// expired entries that are never looked up again.
func (c *TimeCache[K]) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.nowFn()
	for k, expiry := range c.entries {
		if !now.Before(expiry) {
			delete(c.entries, k)
		}
	}
}

// Len returns the total number of entries including expired ones that have
// not yet been cleaned up. Useful for testing and monitoring.
func (c *TimeCache[K]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
