package rpccore

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	NumTimeBuckets = 3
)

// TransactionCache provides a time-bucketed cache for tracking transaction entries with TTL-based eviction.
//
// The cache divides time into 3 fixed-size buckets (NumTimeBuckets), each covering one TTL interval.
// For example, with TTL = 5 minutes, and current time = 12 minutes:
//
//   - Bucket 1 stores entries received between 0–5 min (expired).
//   - Bucket 2 stores entries from 5–10 min (mix of valid and expired).
//   - Bucket 3 stores entries from 10–15 min (valid).
//
// On each tick (i.e., every TTL interval), the active bucket index advances modulo 3.
// For example, after a tick at time = 15 min:
//   - Bucket 1 (previously 0–5 min) now stores entries received between 15–20 min, where all entries are now valid.
//   - Bucket 2 still stores entries received between 5–10 min, but all entries are now completely expired.
//   - Bucket 3 still stores entries received between 10–15 min, but the entries can be either valid or expired.
//
// The cache behaviour is:
//   - `Add()` inserts an entry into the current active bucket (latest time window).
//   - `Contains()` checks the two most recent buckets (excluding the oldest).
//   - On each tick, the evictor deletes the oldest bucket in-place to reclaim memory.
//
// This design enables efficient TTL-based eviction without scanning the entire cache,
// ensuring high-throughput insertion and lookup with bounded memory usage.
type TransactionCache struct {
	ttl           time.Duration
	buckets       [NumTimeBuckets]map[felt.Felt]time.Time // map[txn-hash]entry-time
	locks         [NumTimeBuckets]sync.RWMutex
	curTimeBucket uint32
	tickC         <-chan time.Time
}

// NewTransactionCache creates the cache with a default 1 s eviction ticker.
func NewTransactionCache(ttl time.Duration, cacheSizeHint uint) *TransactionCache {
	cache := &TransactionCache{}
	for b := range NumTimeBuckets {
		cache.buckets[b] = make(map[felt.Felt]time.Time, cacheSizeHint)
	}
	atomic.StoreUint32(&cache.curTimeBucket, 0)
	cache.tickC = time.NewTicker(ttl).C
	cache.ttl = ttl
	return cache
}

// Call this _before_ Run. Should only be used for tests.
func (c *TransactionCache) WithTicker(tickC <-chan time.Time) *TransactionCache {
	c.tickC = tickC
	return c
}

func (c *TransactionCache) Run(ctx context.Context) error {
	c.evictor(ctx)
	return nil
}

func (c *TransactionCache) getCurTimeSlot() uint32 {
	return atomic.LoadUint32(&c.curTimeBucket)
}

func (c *TransactionCache) getPartialExpiredTimeSlot() uint32 {
	cur := atomic.LoadUint32(&c.curTimeBucket)
	return (cur + 2) % NumTimeBuckets
}

func (c *TransactionCache) getExpiredTimeSlot() uint32 {
	cur := atomic.LoadUint32(&c.curTimeBucket)
	return (cur + 1) % NumTimeBuckets
}

func (c *TransactionCache) incrementTimeSlot() {
	old := atomic.LoadUint32(&c.curTimeBucket)
	next := (old + 1) % NumTimeBuckets
	atomic.StoreUint32(&c.curTimeBucket, next)
}

func (c *TransactionCache) Add(key *felt.Felt) {
	timeSlot := c.getCurTimeSlot()
	c.locks[timeSlot].Lock()
	c.buckets[timeSlot][*key] = time.Now()
	c.locks[timeSlot].Unlock()
}

func (c *TransactionCache) Contains(key *felt.Felt) bool {
	for _, b := range [2]uint32{c.getCurTimeSlot(), c.getPartialExpiredTimeSlot()} {
		c.locks[b].RLock()
		insertTime, ok := c.buckets[b][*key]
		c.locks[b].RUnlock()
		if ok {
			return time.Since(insertTime) <= c.ttl
		}
	}
	return false
}

func (c *TransactionCache) evictor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.tickC:
			c.incrementTimeSlot()
			expired := c.getExpiredTimeSlot()
			c.locks[expired].Lock()
			expiredMap := c.buckets[expired]
			for key := range expiredMap {
				delete(expiredMap, key)
			}
			c.locks[expired].Unlock()
		}
	}
}
