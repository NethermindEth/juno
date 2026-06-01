package lru

import (
	"fmt"

	hashilru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/golang-lru/v2/simplelru"
)

// Cache is a thread-safe size-bounded LRU cache.
type Cache[K comparable, V any] struct {
	c *hashilru.Cache[K, V]
}

// New returns a new thread-safe LRU cache or panics if size <= 0.
// Use for caches sized by constants or validated config, where a zero size
// indicates a programmer error rather than a runtime condition.
func New[K comparable, V any](size int) *Cache[K, V] {
	c, err := hashilru.New[K, V](size)
	if err != nil {
		panic(fmt.Errorf("lru: %w (size=%d)", err, size))
	}
	return &Cache[K, V]{c: c}
}

func (l *Cache[K, V]) Add(key K, value V) (evicted bool) { return l.c.Add(key, value) }
func (l *Cache[K, V]) Get(key K) (V, bool)               { return l.c.Get(key) }
func (l *Cache[K, V]) Remove(key K) (present bool)       { return l.c.Remove(key) }
func (l *Cache[K, V]) Purge()                            { l.c.Purge() }
func (l *Cache[K, V]) Len() int                          { return l.c.Len() }

// SimpleCache is a non-thread-safe size-bounded LRU cache.
// Use when external synchronization is provided (e.g. single-goroutine ownership).
type SimpleCache[K comparable, V any] struct {
	c *simplelru.LRU[K, V]
}

// NewSimple returns a new non-thread-safe LRU cache or panics if size <= 0.
func NewSimple[K comparable, V any](size int) *SimpleCache[K, V] {
	c, err := simplelru.NewLRU[K, V](size, nil)
	if err != nil {
		panic(fmt.Errorf("simplelru: %w (size=%d)", err, size))
	}
	return &SimpleCache[K, V]{c: c}
}

func (l *SimpleCache[K, V]) Add(key K, value V) (evicted bool) { return l.c.Add(key, value) }
func (l *SimpleCache[K, V]) Get(key K) (V, bool)               { return l.c.Get(key) }
func (l *SimpleCache[K, V]) Remove(key K) (present bool)       { return l.c.Remove(key) }
func (l *SimpleCache[K, V]) Purge()                            { l.c.Purge() }
func (l *SimpleCache[K, V]) Len() int                          { return l.c.Len() }
