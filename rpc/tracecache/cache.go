package tracecache

import (
	"context"
	"sync"

	"github.com/NethermindEth/juno/utils/lru"
)

// Cache shares immutable results across requests, using a [Lease] to coordinate
// one producer per key while keeping existing results available to readers.
// See the usage example in [Cache.AcquireWithCondition].
type Cache[K comparable, V any] struct {
	mu      sync.Mutex
	records *lru.SimpleCache[K, V]
	flights map[K]chan struct{}
}

// Lease represents a caller's exclusive right to produce and publish a result
// for one [Cache] key. It coordinates replacement of the cached value without
// blocking requests that the existing value already satisfies.
// See the usage example in [Cache.AcquireWithCondition].
type Lease[K comparable, V any] struct {
	cache  *Cache[K, V]
	key    K
	flight chan struct{}
}

type cacheLookupKind uint8

const (
	cacheHit cacheLookupKind = iota
	cacheWait
	cacheLease
)

type cacheLookup[K comparable, V any] struct {
	kind  cacheLookupKind
	value V
	done  <-chan struct{}
	work  *Lease[K, V]
}

func New[K comparable, V any](limit int) *Cache[K, V] {
	return &Cache[K, V]{
		records: lru.NewSimple[K, V](limit),
		flights: make(map[K]chan struct{}),
	}
}

// Acquire returns any cached value or a lease to produce one.
// See [Cache.AcquireWithCondition] for ownership and cancellation requirements.
func (c *Cache[K, V]) Acquire(ctx context.Context, key *K) (V, *Lease[K, V], error) {
	return c.acquire(ctx, key, nil)
}

// AcquireWithCondition returns a cached value, grants a lease, or waits.
//
// Parameters:
//   - ctx: controls cancellation while waiting for another lease to be released.
//   - key: identifies the cached value; must be non-nil.
//   - accepts: a brief, read-only function that reports whether the cached value
//     satisfies this request. Returning true accepts the value without a lease;
//     returning false requests a replacement lease or waits for the current owner.
//     If accepts is nil, any cached value is accepted.
//     It must not call back into this cache.
//
// Outcomes:
//   - A cached value exists and accepts is nil or returns true: return it without a lease.
//   - No cached value and no lease exists: return a lease with the zero value.
//   - accepts returns false and no lease exists: return a lease with the cached value.
//   - A lease exists and no cached value meets the request: wait for its release and retry.
//
// Caller responsibilities:
//   - Cached values aren't deep-copied (as traces can be very large); callers must not
//     mutate shared data they reference.
//   - Call [Lease.Release] to relinquish ownership without changing the cached value,
//     or [Lease.Publish] to store a value and release ownership.
//
// Example usage:
//
//	cached, lease, err := cache.AcquireWithCondition(ctx, &key, accepts)
//	if err != nil {
//	    return nil, err
//	}
//	if lease == nil {
//	    return cached, nil
//	}
//
//	defer lease.Release()
//
//	// buildValue must leave shared cached data unchanged.
//	result, err := buildValue(cached)
//	if err != nil {
//	    return nil, err
//	}
//	lease.Publish(result)
//	return result, nil
func (c *Cache[K, V]) AcquireWithCondition(
	ctx context.Context,
	key *K,
	accepts func(V) bool,
) (V, *Lease[K, V], error) {
	return c.acquire(ctx, key, accepts)
}

func (c *Cache[K, V]) acquire(
	ctx context.Context,
	key *K,
	accepts func(V) bool,
) (V, *Lease[K, V], error) {
	for {
		lookup := c.lookupOrStart(key, accepts)
		switch lookup.kind {
		case cacheHit:
			return lookup.value, nil, nil
		case cacheLease:
			return lookup.value, lookup.work, nil
		case cacheWait:
			select {
			case <-ctx.Done():
				var zero V
				return zero, nil, ctx.Err()
			case <-lookup.done:
			}
		}
	}
}

// lookupOrStart atomically returns a hit, waiter, or replacement lease.
func (c *Cache[K, V]) lookupOrStart(key *K, accepts func(V) bool) cacheLookup[K, V] {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, found := c.records.Get(*key)
	if found && (accepts == nil || accepts(value)) {
		return cacheLookup[K, V]{kind: cacheHit, value: value}
	}
	if flight, found := c.flights[*key]; found {
		return cacheLookup[K, V]{kind: cacheWait, done: flight}
	}
	flight := make(chan struct{})
	c.flights[*key] = flight
	return cacheLookup[K, V]{
		kind:  cacheLease,
		value: value,
		work:  &Lease[K, V]{cache: c, key: *key, flight: flight},
	}
}

func (c *Cache[K, V]) finishLocked(key *K, flight chan struct{}) {
	delete(c.flights, *key)
	close(flight)
}

// Publish stores read-only data and releases ownership. Released leases are ignored.
func (l *Lease[K, V]) Publish(value V) {
	cache := l.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.flights[l.key] != l.flight {
		return
	}
	cache.records.Add(l.key, value)
	cache.finishLocked(&l.key, l.flight)
}

// Release releases ownership without changing the value. Repeated calls are safe.
func (l *Lease[K, V]) Release() {
	cache := l.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.flights[l.key] != l.flight {
		return
	}
	cache.finishLocked(&l.key, l.flight)
}
