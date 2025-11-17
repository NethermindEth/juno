package rpccore

import "sync"

// Cache is expected to at most hold 3 blocks at a time.
// latest block, pre-latest block, and the pre-confirmed block.
const BlockCacheCapacity = 3

// SubscriptionCache is a minimal cache for subscription notifications deduplication.
// Stores notifications by block number and key, with automatic eviction.
//
// The cache is designed to hold at most 3 blocks:
// - up to 2 non-committed blocks (pre-latest and pre-confirmed)
// - and 1 committed block.
// The finalised block will always have the smallest block number.
// When the cache reaches capacity, it evicts the slot with the lowest block number, which always
// corresponds to the oldest finalised block.
type SubscriptionCache[K comparable, V comparable] struct {
	mu    sync.RWMutex
	slots [BlockCacheCapacity]struct {
		blockNum uint64
		data     map[K]V
	}
}

// NewSubscriptionCache creates a new cache.
func NewSubscriptionCache[
	K comparable,
	V comparable,
]() *SubscriptionCache[K, V] {
	cache := &SubscriptionCache[K, V]{}

	for i := range cache.slots {
		cache.slots[i].data = make(map[K]V)
	}
	return cache
}

// Put stores a notification for the given block number and key.
// Evicts the lowest block number when cache is full.
func (c *SubscriptionCache[K, V]) Put(blockNum uint64, key *K, value *V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If block exists, update in-place
	if idx := c.findSlotIndex(blockNum); idx >= 0 {
		c.slots[idx].data[*key] = *value
		return
	}

	slotIdx := c.findSlotToUse()

	clear(c.slots[slotIdx].data)
	c.slots[slotIdx].blockNum = blockNum
	c.slots[slotIdx].data[*key] = *value
}

// ShouldSend returns true if the notification should be sent.
// Returns false if already cached for this block and key.
func (c *SubscriptionCache[K, V]) ShouldSend(blockNum uint64, key *K, value *V) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if idx := c.findSlotIndex(blockNum); idx >= 0 {
		if existingValue, exists := c.slots[idx].data[*key]; exists {
			return existingValue != *value
		}
	}
	return true
}

// Clear removes all cached data.
func (c *SubscriptionCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.slots {
		clear(c.slots[i].data)
		c.slots[i].blockNum = 0
	}
}

// findSlotIndex returns the index of blockNum in c.slots or -1 if absent.
func (c *SubscriptionCache[K, V]) findSlotIndex(blockNum uint64) int {
	for i := range c.slots {
		if len(c.slots[i].data) > 0 && c.slots[i].blockNum == blockNum {
			return i
		}
	}
	return -1
}

// findSlotToUse returns the index of a slot to use for a new block.
// Returns an empty slot if available, otherwise evicts the lowest block number.
func (c *SubscriptionCache[K, V]) findSlotToUse() int {
	// Prefer empty slots (no data) if available
	for i := range c.slots {
		if len(c.slots[i].data) == 0 {
			return i
		}
	}

	// All slots are in use; evict the one with the lowest block number
	lowestIdx := 0
	lowestBlockNum := c.slots[0].blockNum
	for i := 1; i < len(c.slots); i++ {
		if c.slots[i].blockNum < lowestBlockNum {
			lowestIdx = i
			lowestBlockNum = c.slots[i].blockNum
		}
	}
	return lowestIdx
}
