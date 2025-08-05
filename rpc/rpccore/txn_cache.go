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

// The cache divides time into 3 fixed-size buckets (NumTimeBuckets), each representing one TTL interval.
// It tracks a `curTimeBucket` index, which points to the active bucket for new entries.
//
// The three buckets can be interpreted as:
// - Bucket (curTimeBucket - 1) % 3: the previous bucket — entries may be valid or expired, depending on timestamp.
// - Bucket (curTimeBucket    ) % 3: the current bucket — new entries are added here.
// - Bucket (curTimeBucket + 1) % 3: the next bucket — cleared by the evictor-routine before reuse.
//
// On every tick (i.e., TTL duration), the index advances and the evictor clears the oldest bucket in-place.
type TransactionCache struct {
	ttl           time.Duration
	buckets       [NumTimeBuckets]map[felt.Felt]time.Time // map[txn-hash]entry-time
	locks         [NumTimeBuckets]sync.RWMutex
	curTimeBucket atomic.Uint32
	tickC         <-chan time.Time
}

// NewTransactionCache creates the cache with a default 1 s eviction ticker.
func NewTransactionCache(ttl time.Duration, cacheSizeHint uint) *TransactionCache {
	cache := &TransactionCache{}
	for b := range NumTimeBuckets {
		cache.buckets[b] = make(map[felt.Felt]time.Time, cacheSizeHint)
	}
	cache.curTimeBucket.Store(0)
	cache.tickC = time.NewTicker(ttl).C
	cache.ttl = ttl
	return cache
}

func (c *TransactionCache) Run(ctx context.Context) error {
	c.evictor(ctx)
	return nil
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

// Call this _before_ Run. Should only be used for tests.
func (c *TransactionCache) WithTicker(tickC <-chan time.Time) *TransactionCache {
	c.tickC = tickC
	return c
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

func (c *TransactionCache) getCurTimeSlot() uint32 {
	return c.curTimeBucket.Load()
}

func (c *TransactionCache) getPartialExpiredTimeSlot() uint32 {
	cur := c.curTimeBucket.Load()
	return (cur + 2) % NumTimeBuckets
}

func (c *TransactionCache) getExpiredTimeSlot() uint32 {
	cur := c.curTimeBucket.Load()
	return (cur + 1) % NumTimeBuckets
}

func (c *TransactionCache) incrementTimeSlot() {
	old := c.curTimeBucket.Load()
	next := (old + 1) % NumTimeBuckets
	c.curTimeBucket.Store(next)
}
