package timecache

import (
	"sync"
	"time"
)

type index int

type timedValue[K any] struct {
	value  K
	expiry time.Time
}

type TimeCache[K comparable] struct {
	// Access valid keys in O(1)
	values map[K]time.Time
	// Expire in O(k) where `k` is the amount of expired keys
	timestamps []timedValue[K]
	mu         sync.RWMutex

	// Track currently stored values, first value at `start` and last
	// one at `end`. `size` is the maximum amount of elements we can hold
	start index
	end   index
	size  int
	// used to delete any timed value which has been inserted more than
	// `exipiry` time.Duration ago.
	expiry time.Duration
}

// New allocates a new Timecache with initial allocation size and expiry time.
// If `size` gets filled the timecache will allocate more memory to fit more
// elements into it. The cache will not shrink after regrowing.
func New[K comparable](size int, expiry time.Duration) *TimeCache[K] {
	// we allocate size+1 because we allways leave the last position empty
	// to detect when the cache is full
	return &TimeCache[K]{
		values:     make(map[K]time.Time, size+1),
		timestamps: make([]timedValue[K], size+1),
		mu:         sync.RWMutex{},

		start:  0,
		end:    0,
		size:   size + 1,
		expiry: expiry,
	}
}

// Add adds a new key into the timecache, it doesn't guard against duplicated
// entries. Adding the same entry twice will result in undefined behaviour.
func (tc *TimeCache[K]) Add(value *K) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	now := time.Now()
	tc.removeExpired(now)
	if tc.almostFull() {
		tc.regrowth()
	}

	expiryTime := now.Add(tc.expiry)
	tc.values[*value] = expiryTime
	tc.timestamps[tc.end] = timedValue[K]{
		value:  *value,
		expiry: expiryTime,
	}
	tc.increaseIndex(&tc.end)
}

func (tc *TimeCache[K]) Get(value *K) bool {
	tc.mu.RLock()
	expiry, ok := tc.values[*value]
	tc.mu.RUnlock()

	if !ok {
		return false
	}

	now := time.Now()
	if expiry.After(now) {
		return true
	}

	// If we know we have an expired value
	// let's clean the expired entries
	tc.mu.Lock()
	tc.removeExpired(now)
	tc.mu.Unlock()
	return false
}

func (tc *TimeCache[K]) increaseIndex(idx *index) {
	*idx = (*idx + 1) % index(tc.size)
}

// removeExpired deletes all the elements that have already expired until it
// finds the first one that hasn't or the cache empties
func (tc *TimeCache[K]) removeExpired(now time.Time) {
	for tc.start != tc.end {
		tv := tc.timestamps[tc.start]
		if now.Before(tv.expiry) {
			break
		}

		delete(tc.values, tv.value)
		tc.increaseIndex(&tc.start)
	}
}

// almostFull returns if the time cache will get full on the next insertion
func (tc *TimeCache[K]) almostFull() bool {
	nextEnd := tc.end
	tc.increaseIndex(&nextEnd)
	return nextEnd == tc.start
}

func (tc *TimeCache[K]) regrowth() {
	const standardSize = 1024

	nextSize := tc.size * 2
	if tc.size > standardSize {
		// growth by 20%
		nextSize = (tc.size * 12) / 10
	}

	nextTimestamps := make([]timedValue[K], nextSize)

	// This case only applies when start == 0 and end == size-1
	if tc.start < tc.end {
		copy(nextTimestamps, tc.timestamps)
		tc.size = nextSize
		tc.timestamps = nextTimestamps
		return
	}

	count := tc.size - int(tc.start)
	copy(nextTimestamps[0:count], tc.timestamps[tc.start:tc.size])
	nextEnd := count + int(tc.end)
	copy(nextTimestamps[count:nextEnd], tc.timestamps[0:tc.end])

	tc.start = 0
	tc.end = index(count) + tc.end
	tc.size = nextSize
	tc.timestamps = nextTimestamps
}
