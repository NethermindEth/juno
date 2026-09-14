package tracecache

import (
	"context"
	"sync"

	"github.com/NethermindEth/juno/utils/lru"
)

// Cache stores immutable values with one active owner per key.
// Callers define key validity and produce values outside the mutex.
type Cache[K comparable, V any] struct {
	mu      sync.Mutex
	records *lru.SimpleCache[K, V]
	flights map[K]chan struct{}
}

type cacheLookupKind uint8

const (
	// Reserve zero so an uninitialized lookup is not treated as a cache hit.
	cacheHit cacheLookupKind = iota + 1
	cacheWait
	cacheLoad
)

type cacheLookup[K comparable, V any] struct {
	kind  cacheLookupKind
	value V // current value, also returned to replacement owners
	done  <-chan struct{}
	work  *Lease[K, V]
}

// Lease is the exclusive right to publish for a key, independent of eviction.
type Lease[K comparable, V any] struct {
	cache  *Cache[K, V]
	key    K
	flight chan struct{}
}

func New[K comparable, V any](limit int) *Cache[K, V] {
	return &Cache[K, V]{
		records: lru.NewSimple[K, V](limit),
		flights: make(map[K]chan struct{}),
	}
}

// lookupOrStart atomically returns a hit, waiter, or replacement lease.
func (c *Cache[K, V]) lookupOrStart(key K, accepts func(V) bool) cacheLookup[K, V] {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, found := c.records.Get(key)
	if found && (accepts == nil || accepts(value)) {
		return cacheLookup[K, V]{kind: cacheHit, value: value}
	}
	if flight, found := c.flights[key]; found {
		return cacheLookup[K, V]{kind: cacheWait, done: flight}
	}
	flight := make(chan struct{})
	c.flights[key] = flight
	return cacheLookup[K, V]{
		kind: cacheLoad, value: value,
		work: &Lease[K, V]{cache: c, key: key, flight: flight},
	}
}

// Publish replaces the value and releases ownership. Published data must remain read-only.
// Calls on released leases have no effect.
func (w *Lease[K, V]) Publish(value V) {
	cache := w.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.flights[w.key] != w.flight {
		return
	}
	cache.records.Add(w.key, value)
	cache.finishLocked(w.key, w.flight)
}

// Abort releases ownership without changing the value. Repeated calls are safe.
func (w *Lease[K, V]) Abort() {
	cache := w.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.flights[w.key] != w.flight {
		return
	}
	cache.finishLocked(w.key, w.flight)
}

func (c *Cache[K, V]) finishLocked(key K, flight chan struct{}) {
	delete(c.flights, key)
	close(flight)
}

// Acquire returns an accepted value immediately, even during replacement; otherwise
// it waits for an owner or returns a lease. Defer Abort on any returned lease.
// A nil accepts allows any cached value; otherwise it runs under the mutex and
// must be brief, read-only, and never reenter the cache.
// Failed owners release waiters to retry. Cancellation only stops this caller's wait.
func (c *Cache[K, V]) Acquire(ctx context.Context, key K, accepts func(V) bool) (V, *Lease[K, V], error) {
	for {
		lookup := c.lookupOrStart(key, accepts)
		switch lookup.kind {
		case cacheHit:
			return lookup.value, nil, nil
		case cacheLoad:
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
