package rpccore

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

// Todo: there is an issue with Contains. Sometimes gives fasle resutls

const (
	mapCapacityHint = 1024  // Assuming 1024 TPS
	NumTimeBuckets  = 5 + 1 // TTL is 5s, plus 1 bucket for expired entries.
)

// SubmittedTransactionsCacheAlt implements a fixed‑TTL, time‑wheel in-memory cache for txn hashes.
// It divides time into NumTimeBuckets slots and further shards each slot into numLocks maps to
// minimize lock contention. Entries live for `NumTimeBuckets-1`, after which the oldest slot
// is automatically cleared on each tick/second, evicting those entries.
//
// `NumTimeBuckets` defines the total number of time slots,
// and `numLocks` defines the number of lock‑stripes per slot.
type SubmittedTransactionsCacheAlt struct {
	buckets       [NumTimeBuckets]map[felt.Felt]struct{} // Stores the presence of the txn hash
	locks         [NumTimeBuckets]sync.RWMutex
	tick          time.Duration
	curTimeBucket uint32
	stop          chan struct{}
}

// NewSubmittedTransactionsCacheAlt creates the cache with a default 1 s eviction ticker.
func NewSubmittedTransactionsCacheAlt() *SubmittedTransactionsCacheAlt {
	return NewSubmittedTransactionsCacheAltWithTicker(time.NewTicker(time.Second).C)
}

// NewSubmittedTransactionsCacheAltWithTicker creates the cache and starts eviction driven by the provided ticker channel.
// Useful for injecting a fake clock in tests.
func NewSubmittedTransactionsCacheAltWithTicker(tickC <-chan time.Time) *SubmittedTransactionsCacheAlt {
	c := &SubmittedTransactionsCacheAlt{
		stop: make(chan struct{}),
	}
	for b := 0; b < NumTimeBuckets; b++ {
		c.buckets[b] = make(map[felt.Felt]struct{}, mapCapacityHint)
	}
	atomic.StoreUint32(&c.curTimeBucket, 0)
	go c.evictor(tickC)
	return c
}

func (c *SubmittedTransactionsCacheAlt) getCurTimeSlot() uint32 {
	return atomic.LoadUint32(&c.curTimeBucket)
}

func (c *SubmittedTransactionsCacheAlt) getExpiredTimeSlot() uint32 {
	cur := atomic.LoadUint32(&c.curTimeBucket)
	return (cur + 1) % NumTimeBuckets
}

func (c *SubmittedTransactionsCacheAlt) incrementTimeSlot() {
	old := atomic.LoadUint32(&c.curTimeBucket)
	next := (old + 1) % NumTimeBuckets
	atomic.StoreUint32(&c.curTimeBucket, next)
}

// Todo: just a note.
// Old approach. Lock entire map. Flush performs O(n) loop. Add to map. Add to list. Push to front.
// New approach. Contains() call. Lock section of the map (timeslot, lockgroup). O(1) set.
func (c *SubmittedTransactionsCacheAlt) Set(key felt.Felt) {
	if c.Contains(key) {
		return
	}

	timeSlot := c.getCurTimeSlot()
	c.locks[timeSlot].Lock()
	c.buckets[timeSlot][key] = struct{}{}
	c.locks[timeSlot].Unlock()
}

// Todo: just a note.
// Old approach. Lock entire map. O(1) lookup. Potentially update map. Modify underlying map and list.
// New approach. O(1) lookup. Lock part of the map. Only read lock. Don't modify the map.
func (c *SubmittedTransactionsCacheAlt) Contains(key felt.Felt) bool {
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

// Todo: just a note.
// Old approach. Try and evict every time we add to the map.
// New approach. Evict in time-buckets, once per tick. Shouldn't contend with locks in Set/Get, since operates on seperate bucket.
func (c *SubmittedTransactionsCacheAlt) evictor(tickC <-chan time.Time) {
	for {
		select {
		case <-tickC:
			// New entries should be immediately directed to a new bucket
			// which should already be empty
			c.incrementTimeSlot()
			// Clean up the next bucket
			expired := c.getExpiredTimeSlot()
			// Clean map in-place. This is signifcaintly more efficient than allocating a new map.
			c.locks[expired].Lock()
			m := c.buckets[expired]
			for k := range m {
				delete(m, k)
			}
			c.locks[expired].Unlock()

		case <-c.stop:
			return
		}
	}
}

func (c *SubmittedTransactionsCacheAlt) Stop() {
	close(c.stop)
}
