package tracecache

import (
	"context"
	"sync"

	"github.com/NethermindEth/juno/utils/lru"
)

// Cache stores immutable values with one active owner per key.
type Cache[K comparable, V any] struct {
	mu      sync.Mutex
	records *lru.SimpleCache[K, V]
	flights map[K]chan struct{}
}

// Lease is the exclusive right to publish for a key, independent of eviction.
type Lease[K comparable, V any] struct {
	cache  *Cache[K, V]
	key    K
	flight chan struct{}
}

type cacheLookupKind uint8

const (
	cacheHit cacheLookupKind = iota
	cacheWait
	cacheLoad
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

// Acquire returns an accepted hit or a lease with the old value (zero on a miss).
// Hits bypass active owners; otherwise callers wait for them. Defer Abort on returned leases.
// Nil accepts allows any cached value; predicates must be brief, read-only, and never reenter.
// Lookups and predicates hold the cache mutex; waiting and value production do not.
// Cancellation only stops this caller's wait.
// Key must be non-nil and remain unchanged until Acquire returns.
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
		kind: cacheLoad, value: value,
		work: &Lease[K, V]{cache: c, key: *key, flight: flight},
	}
}

func (c *Cache[K, V]) finishLocked(key *K, flight chan struct{}) {
	delete(c.flights, *key)
	close(flight)
}

// Publish stores read-only data and releases ownership. Released leases are ignored.
func (w *Lease[K, V]) Publish(value V) {
	cache := w.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.flights[w.key] != w.flight {
		return
	}
	cache.records.Add(w.key, value)
	cache.finishLocked(&w.key, w.flight)
}

// Abort releases ownership without changing the value. Repeated calls are safe.
func (w *Lease[K, V]) Abort() {
	cache := w.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.flights[w.key] != w.flight {
		return
	}
	cache.finishLocked(&w.key, w.flight)
}
