package rpccore

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

const (
	numBuckets = 5   // TTL is 5s, so we bucket into 5 groups.
	numShards  = 256 // Todo: investigate
)

type SubmittedTransactionsCacheAlt struct {
	buckets   [numBuckets][numShards]map[*felt.Felt]time.Time
	locks     [numBuckets][numShards]sync.RWMutex
	tick      time.Duration
	curBucket uint32
	stop      chan struct{}
}

func NewSubmittedTransactionsCacheAlt(tick time.Duration) *SubmittedTransactionsCacheAlt {
	c := &SubmittedTransactionsCacheAlt{
		tick: tick,
		stop: make(chan struct{}),
	}
	for b := 0; b < numBuckets; b++ {
		for s := 0; s < numShards; s++ {
			c.buckets[b][s] = make(map[*felt.Felt]time.Time)
		}
	}
	atomic.StoreUint32(&c.curBucket, 0)
	go c.evictor()
	return c
}

func (c *SubmittedTransactionsCacheAlt) getCurTimeSlot() int {
	return int(atomic.LoadUint32(&c.curBucket))
}

func (c *SubmittedTransactionsCacheAlt) Set(key *felt.Felt, ts time.Time) {
	timeSlot := c.getCurTimeSlot()
	lockGroupID := int(key.Uint64() % numShards)
	c.locks[timeSlot][lockGroupID].Lock()
	c.buckets[timeSlot][lockGroupID][key] = ts
	c.locks[timeSlot][lockGroupID].Unlock()
}

func (c *SubmittedTransactionsCacheAlt) Get(key *felt.Felt) (time.Time, bool) {
	lockGroupID := int(key.Uint64() % numShards)
	for b := 0; b < numBuckets; b++ {
		c.locks[b][lockGroupID].RLock()
		ts, ok := c.buckets[b][lockGroupID][key]
		c.locks[b][lockGroupID].RUnlock()
		if ok {
			return ts, true
		}
	}
	return time.Time{}, false
}

func (c *SubmittedTransactionsCacheAlt) evictor() {
	ticker := time.NewTicker(c.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			next := (c.getCurTimeSlot() + 1) % numBuckets
			for s := 0; s < numShards; s++ {
				c.locks[next][s].Lock()
				c.buckets[next][s] = make(map[*felt.Felt]time.Time)
				c.locks[next][s].Unlock()
			}
			atomic.StoreUint32(&c.curBucket, uint32(next))
		case <-c.stop:
			return
		}
	}
}

func (c *SubmittedTransactionsCacheAlt) Stop() {
	close(c.stop)
}
