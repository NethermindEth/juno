package rpccore

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

// Todo: make sure we don't try and get form the bucket that is being deleted
// Todo: investigate if the lock groups are actually useful

const (
	mapCapacityHint = 1024  // Assuming 1024 TPS
	NumTimeBuckets  = 5 + 1 // TTL is 5s
	numLocks        = 1     // Todo: investigate. Increasing seems to drive down ns/op (lock contention), and B/op. but increases allocs/op ?????? Unless rand uint64 ???
)

// SubmittedTransactionsCacheAlt implements a fixed‑TTL, time‑wheel in-memory cache for txn hashes.
// It divides time into NumTimeBuckets slots and further shards each slot into numLocks maps to
// minimize lock contention. Entries live for `NumTimeBuckets-1`, after which the oldest slot
// is automatically cleared on each tick/second, evicting those entries.
//
// `NumTimeBuckets` defines the total number of time slots,
// and `numLocks` defines the number of lock‑stripes per slot.
type SubmittedTransactionsCacheAlt struct {
	buckets   [NumTimeBuckets][numLocks]map[felt.Felt]struct{} // Stores the presence of the txn hash
	locks     [NumTimeBuckets][numLocks]sync.RWMutex
	tick      time.Duration
	curBucket uint32
	stop      chan struct{}
}

// NewSubmittedTransactionsCacheAlt creates the cache with per‑shard pre‑allocation and starts eviction.
func NewSubmittedTransactionsCacheAlt(tickC <-chan time.Time) *SubmittedTransactionsCacheAlt {
	c := &SubmittedTransactionsCacheAlt{
		stop: make(chan struct{}),
	}
	for b := 0; b < NumTimeBuckets; b++ {
		for s := 0; s < numLocks; s++ {
			c.buckets[b][s] = make(map[felt.Felt]struct{}, mapCapacityHint)
		}
	}
	atomic.StoreUint32(&c.curBucket, 0)
	go c.evictor(tickC)
	return c
}

func (c *SubmittedTransactionsCacheAlt) getCurTimeSlot() uint32 {
	return atomic.LoadUint32(&c.curBucket)
}

// Todo: just a note.
// Old approach. Lock entire map. Flush performs O(n) loop. Add to map. Add to list. Push to front.
// New approach. Contains() call. Lock section of the map (timeslot, lockgroup). O(1) set.
func (c *SubmittedTransactionsCacheAlt) Set(key felt.Felt) {
	if c.Contains(key) {
		return
	}

	timeSlot := c.getCurTimeSlot()
	lockGroupID := int(key.Uint64() % numLocks)
	c.locks[timeSlot][lockGroupID].Lock()
	c.buckets[timeSlot][lockGroupID][key] = struct{}{}
	c.locks[timeSlot][lockGroupID].Unlock()
}

// Todo: just a note.
// Old approach. Lock entire map. O(1) lookup. Potentially update map. Modify underlying map and list.
// New approach. O(1) lookup. Lock part of the map. Only read lock. Don't modify the map.
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

// Todo: just a note.
// Old approach. Try and evict every time we add to the map.
// New approach. Evict in time-buckets, once per tick. Shouldn't contend with locks in Set/Get, since operates on seperate bucket.
func (c *SubmittedTransactionsCacheAlt) evictor(tickC <-chan time.Time) {
	for {
		select {
		case <-tickC:
			nextTimeBucket := (c.getCurTimeSlot() + 1) % NumTimeBuckets
			// Clean map in-place. This is signifcaintly more efficient than allocating a new map.
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
