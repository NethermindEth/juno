package rpccore

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	NumTimeBuckets = 5 + 1 // TTL is 5s
	numShards      = 256   // Todo: investigate
)

type SubmittedTransactionsCacheAlt struct {
	buckets   [NumTimeBuckets][numShards]map[*felt.Felt]struct{} // Stores the presence of the txn hash
	locks     [NumTimeBuckets][numShards]sync.RWMutex
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
		for s := 0; s < numShards; s++ {
			c.buckets[b][s] = make(map[*felt.Felt]struct{})
		}
	}
	atomic.StoreUint32(&c.curBucket, 0)
	go c.evictor(tickC)
	return c
}

func (c *SubmittedTransactionsCacheAlt) getCurTimeSlot() uint32 {
	return atomic.LoadUint32(&c.curBucket)
}

func (c *SubmittedTransactionsCacheAlt) Set(key *felt.Felt) {
	timeSlot := c.getCurTimeSlot()
	lockGroupID := int(key.Uint64() % numShards)
	c.locks[timeSlot][lockGroupID].Lock()
	c.buckets[timeSlot][lockGroupID][key] = struct{}{} // Todo: by resubmitting the txn, we can get duplicates..
	c.locks[timeSlot][lockGroupID].Unlock()
}

func (c *SubmittedTransactionsCacheAlt) Contains(key *felt.Felt) bool {
	lockGroupID := int(key.Uint64() % numShards)
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
			next := (c.getCurTimeSlot() + 1) % NumTimeBuckets
			for s := 0; s < numShards; s++ {
				c.locks[next][s].Lock()
				c.buckets[next][s] = make(map[*felt.Felt]struct{})
				c.locks[next][s].Unlock()
			}
			atomic.StoreUint32(&c.curBucket, next)
		case <-c.stop:
			return
		}
	}
}

func (c *SubmittedTransactionsCacheAlt) Stop() {
	close(c.stop)
}
