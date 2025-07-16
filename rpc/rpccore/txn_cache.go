package rpccore

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	mapCapacityHint = 1024  // Assuming 1024 TPS
	NumTimeBuckets  = 5 + 1 // TTL is 5s. 1 additional bucket for outdated entries.
)

type TxnCache struct {
	buckets       [NumTimeBuckets]map[felt.Felt]struct{} // Stores the presence of the txn hash
	locks         [NumTimeBuckets]sync.RWMutex
	curTimeBucket uint32
	tickC         <-chan time.Time
}

// NewTxnCache creates the cache with a default 1 s eviction ticker.
func NewTxnCache() *TxnCache {
	cache := &TxnCache{}
	for b := range NumTimeBuckets {
		cache.buckets[b] = make(map[felt.Felt]struct{}, mapCapacityHint)
	}
	atomic.StoreUint32(&cache.curTimeBucket, 0)
	cache.tickC = time.NewTicker(time.Second).C
	return cache
}

// Call this _before_ Run. Should only be used for tests.
func (c *TxnCache) WithTicker(tickC <-chan time.Time) *TxnCache {
	c.tickC = tickC
	return c
}

func (c *TxnCache) Run(ctx context.Context) error {
	c.evictor(ctx)
	return nil
}

func (c *TxnCache) getCurTimeSlot() uint32 {
	return atomic.LoadUint32(&c.curTimeBucket)
}

func (c *TxnCache) getExpiredTimeSlot() uint32 {
	cur := atomic.LoadUint32(&c.curTimeBucket)
	return (cur + 1) % NumTimeBuckets
}

func (c *TxnCache) incrementTimeSlot() {
	old := atomic.LoadUint32(&c.curTimeBucket)
	next := (old + 1) % NumTimeBuckets
	atomic.StoreUint32(&c.curTimeBucket, next)
}

func (c *TxnCache) Add(key felt.Felt) {
	if c.Contains(key) {
		return
	}

	timeSlot := c.getCurTimeSlot()
	c.locks[timeSlot].Lock()
	c.buckets[timeSlot][key] = struct{}{}
	c.locks[timeSlot].Unlock()
}

func (c *TxnCache) Contains(key felt.Felt) bool {
	expired := c.getExpiredTimeSlot()

	for b := range uint32(NumTimeBuckets) {
		if b == expired {
			continue
		}
		c.locks[b].RLock()
		_, ok := c.buckets[b][key]
		c.locks[b].RUnlock()
		if ok {
			return true
		}
	}
	return false
}

func (c *TxnCache) evictor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.tickC:
			c.incrementTimeSlot()

			expired := c.getExpiredTimeSlot()
			c.locks[expired].Lock()
			m := c.buckets[expired]
			for k := range m {
				delete(m, k)
			}
			c.locks[expired].Unlock()
		}
	}
}
