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
}

// RunTxnCache creates the cache with a default 1 s eviction ticker.
func RunTxnCache(ctx context.Context) *TxnCache {
	return RunTxnCacheWithTicker(ctx, time.NewTicker(time.Second).C)
}

// Useful for injecting a fake clock in tests.
func RunTxnCacheWithTicker(ctx context.Context, tickC <-chan time.Time) *TxnCache {
	c := &TxnCache{}
	for b := 0; b < NumTimeBuckets; b++ {
		c.buckets[b] = make(map[felt.Felt]struct{}, mapCapacityHint)
	}
	atomic.StoreUint32(&c.curTimeBucket, 0)
	go c.evictor(ctx, tickC)
	return c
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

func (c *TxnCache) Set(key felt.Felt) {
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

	for b := uint32(0); b < NumTimeBuckets; b++ {
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

func (c *TxnCache) evictor(ctx context.Context, tickC <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-tickC:
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
