package rpccore

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	NumTimeBuckets = 5 + 1 // TTL is 5s
	numLocks       = 256   // Todo: investigate
)

type SubmittedTransactionsCacheAlt struct {
	buckets   [NumTimeBuckets][numLocks]map[felt.Felt]struct{} // Stores the presence of the txn hash
	locks     [NumTimeBuckets][numLocks]sync.RWMutex
	tick      time.Duration
	curBucket uint32
	stop      chan struct{}
}

// NewSubmittedTransactionsCacheAlt creates the cache and starts eviction driven by tickC.
func NewSubmittedTransactionsCacheAlt(tickC <-chan time.Time) *SubmittedTransactionsCacheAlt {
	c := &SubmittedTransactionsCacheAlt{
		stop: make(chan struct{}),
	}
	for b := 0; b < NumTimeBuckets; b++ {
		for s := 0; s < numLocks; s++ {
			c.buckets[b][s] = make(map[felt.Felt]struct{})
		}
	}
	atomic.StoreUint32(&c.curBucket, 0)
	go c.evictor(tickC)
	return c
}

func (c *SubmittedTransactionsCacheAlt) getCurTimeSlot() uint32 {
	return atomic.LoadUint32(&c.curBucket)
}

func (c *SubmittedTransactionsCacheAlt) Set(key felt.Felt) {
	// This prevents duplicates
	// Note: this doubled `ns/op` in the benchmarks indicating lock contention.
	// We only get duplciates if the user sends the same txn into differnt time buckets (eg max NumTimeBuckets)
	// This causes the txn to live in the cahce for longer, which is only a UX problem if the user queries and the
	// FGW hasn't updated its status. ie it may be acceptable to drop this?
	if c.Contains(key) {
		return
	}

	timeSlot := c.getCurTimeSlot()
	lockGroupID := int(key.Uint64() % numLocks)
	c.locks[timeSlot][lockGroupID].Lock()
	c.buckets[timeSlot][lockGroupID][key] = struct{}{}
	c.locks[timeSlot][lockGroupID].Unlock()
}

func (c *SubmittedTransactionsCacheAlt) Contains(key felt.Felt) bool {
	lockGroupID := int(key.Uint64() % numLocks)
	for b := 0; b < NumTimeBuckets; b++ {
		c.locks[b][lockGroupID].RLock()
		_, ok := c.buckets[b][lockGroupID][key]
		c.locks[b][lockGroupID].RUnlock()
		if ok {
			return true
		}
	}
	return false
}

func (c *SubmittedTransactionsCacheAlt) evictor(tickC <-chan time.Time) {
	for {
		select {
		case <-tickC:
			nextTimeBucket := (c.getCurTimeSlot() + 1) % NumTimeBuckets
			// Clean map in place.
			for lockID := 0; lockID < numLocks; lockID++ {
				c.locks[nextTimeBucket][lockID].Lock()
				m := c.buckets[nextTimeBucket][lockID]
				for k := range m {
					delete(m, k)
				}
				c.locks[nextTimeBucket][lockID].Unlock()
			}
			atomic.StoreUint32(&c.curBucket, nextTimeBucket)
		case <-c.stop:
			return
		}
	}
}

func (c *SubmittedTransactionsCacheAlt) Stop() {
	close(c.stop)
}
