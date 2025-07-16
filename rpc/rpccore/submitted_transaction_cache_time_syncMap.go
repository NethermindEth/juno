package rpccore

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

type SubmittedTransactionsCacheSyncMap struct {
	buckets   [NumTimeBuckets]atomic.Value // each holds a *sync.Map
	tick      time.Duration
	curBucket uint32
	stop      chan struct{}
}

func NewSubmittedTransactionsCacheSyncMap(tickC <-chan time.Time) *SubmittedTransactionsCacheSyncMap {
	c := &SubmittedTransactionsCacheSyncMap{
		tick: tickCToDuration(tickC), // helper to extract duration if needed
		stop: make(chan struct{}),
	}
	for i := 0; i < NumTimeBuckets; i++ {
		c.buckets[i].Store(new(sync.Map))
	}
	atomic.StoreUint32(&c.curBucket, 0)
	go c.evictor(tickC)
	return c
}

// Set records presence of key in the current time slot.
func (c *SubmittedTransactionsCacheSyncMap) Set(key felt.Felt) {
	if c.Contains(key) {
		return
	}
	idx := c.getCurTimeSlot()
	m := c.buckets[idx].Load().(*sync.Map)
	m.Store(key, struct{}{})
}

// Contains reports whether key is present in any non‑evicted bucket.
func (c *SubmittedTransactionsCacheSyncMap) Contains(key felt.Felt) bool {
	for i := 0; i < NumTimeBuckets; i++ {
		m := c.buckets[i].Load().(*sync.Map)
		if _, ok := m.Load(key); ok {
			return true
		}
	}
	return false
}

// evictor advances the wheel on each tickC and replaces the oldest bucket.
func (c *SubmittedTransactionsCacheSyncMap) evictor(tickC <-chan time.Time) {
	for {
		select {
		case <-tickC:
			next := (c.getCurTimeSlot() + 1) % NumTimeBuckets
			m := c.buckets[next].Load().(*sync.Map)
			m.Range(func(key, _ interface{}) bool {
				m.Delete(key)
				return true
			})
			atomic.StoreUint32(&c.curBucket, uint32(next))
		case <-c.stop:
			return
		}
	}
}

// Stop halts eviction.
func (c *SubmittedTransactionsCacheSyncMap) Stop() {
	close(c.stop)
}

// getCurTimeSlot returns the current active slot index.
func (c *SubmittedTransactionsCacheSyncMap) getCurTimeSlot() int {
	return int(atomic.LoadUint32(&c.curBucket) % NumTimeBuckets)
}

// tickCToDuration is a placeholder if you need to record tick intervals;
// otherwise you can drop tick from the struct or pass the duration separately.
func tickCToDuration(_ <-chan time.Time) time.Duration {
	// not used in sync.Map version, eviction driven by channel
	return 0
}
