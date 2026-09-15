package tracecache

import (
	"context"
	"sync"

	"github.com/NethermindEth/juno/utils/lru"
)

// Cache shares immutable results across requests, using a [Lease] to coordinate
// one producer per key while keeping existing results available to readers.
// See the usage example in [Cache.Acquire].
type Cache[K comparable, V any] struct {
	mu      sync.Mutex
	records *lru.SimpleCache[K, V]
	flights map[K]chan struct{}
}

// Lease represents a caller's exclusive right to produce and publish a result
// for one [Cache] key. It coordinates replacement of the cached value without
// blocking requests that the existing value already satisfies.
// See the usage example in [Cache.Acquire].
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

// Acquire returns a cached value, grants a lease, or waits:
//   - A cached value exists and accepts is nil or returns true: return it without a lease.
//   - No cached value and no lease exists: return a lease with the zero value.
//   - accepts returns false and no lease exists: return a lease with the cached value.
//   - A lease exists and no cached value meets the request: wait for its release and retry.
//
// Caller responsibilities:
//   - Cached values aren't deep-copied (as traces can be very large); callers must not
//     mutate shared data they reference.
//   - Callers must defer [Lease.Abort] on returned leases and call [Lease.Publish] on success.
//   - accepts must be brief, read-only, and must not call back into this cache.
//   - key must be non-nil (nil panics); callers must not mutate the value it
//     points to until Acquire returns.
//
// lookupOrStart and accepts briefly hold the cache mutex.
// Waiting and caller execution happen outside the mutex.
// Cancellation only stops this caller's wait.
//
// Example usage:
//
//	cached, lease, err := cache.Acquire(ctx, &key, accepts)
//	if err != nil {
//	    return nil, err
//	}
//	if lease == nil {
//	    return cached, nil
//	}
//
//	defer lease.Abort()
//
//	// produce must leave shared cached data unchanged.
//	result, err := produce(cached)
//	if err != nil {
//	    return nil, err
//	}
//	lease.Publish(result)
//	return result, nil
func (c *Cache[K, V]) Acquire(
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

// Abort releases ownership without changing the value. Repeated calls are safe.
func (l *Lease[K, V]) Abort() {
	cache := l.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.flights[l.key] != l.flight {
		return
	}
	cache.finishLocked(&l.key, l.flight)
}
