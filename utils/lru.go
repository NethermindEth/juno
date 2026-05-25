package utils

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/golang-lru/v2/simplelru"
)

// LRU is a thread-safe size-bounded LRU cache.
type LRU[K comparable, V any] struct {
	c *lru.Cache[K, V]
}

// NewLRU returns a new thread-safe LRU cache or panics if size <= 0.
// Use for caches sized by constants or validated config, where a zero size
// indicates a programmer error rather than a runtime condition.
func NewLRU[K comparable, V any](size int) *LRU[K, V] {
	c, err := lru.New[K, V](size)
	if err != nil {
		panic(fmt.Errorf("lru: %w (size=%d)", err, size))
	}
	return &LRU[K, V]{c: c}
}

func (l *LRU[K, V]) Add(key K, value V) (evicted bool) { return l.c.Add(key, value) }
func (l *LRU[K, V]) Get(key K) (V, bool)               { return l.c.Get(key) }
func (l *LRU[K, V]) Remove(key K) (present bool)       { return l.c.Remove(key) }
func (l *LRU[K, V]) Purge()                            { l.c.Purge() }
func (l *LRU[K, V]) Len() int                          { return l.c.Len() }

// SimpleLRU is a non-thread-safe size-bounded LRU cache.
// Use when external synchronization is provided (e.g. single-goroutine ownership).
type SimpleLRU[K comparable, V any] struct {
	c *simplelru.LRU[K, V]
}

// NewSimpleLRU returns a new non-thread-safe LRU cache or panics if size <= 0.
func NewSimpleLRU[K comparable, V any](size int) *SimpleLRU[K, V] {
	c, err := simplelru.NewLRU[K, V](size, nil)
	if err != nil {
		panic(fmt.Errorf("simplelru: %w (size=%d)", err, size))
	}
	return &SimpleLRU[K, V]{c: c}
}

func (l *SimpleLRU[K, V]) Add(key K, value V) (evicted bool) { return l.c.Add(key, value) }
func (l *SimpleLRU[K, V]) Get(key K) (V, bool)               { return l.c.Get(key) }
func (l *SimpleLRU[K, V]) Remove(key K) (present bool)       { return l.c.Remove(key) }
func (l *SimpleLRU[K, V]) Purge()                            { l.c.Purge() }
func (l *SimpleLRU[K, V]) Len() int                          { return l.c.Len() }
