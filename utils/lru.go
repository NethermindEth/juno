package utils

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/hashicorp/golang-lru/v2/simplelru"
)

// NewLRU returns a new thread-safe LRU cache or panics if size <= 0.
// Use for caches sized by constants or validated config, where a zero size
// indicates a programmer error rather than a runtime condition.
func NewLRU[K comparable, V any](size int) *lru.Cache[K, V] {
	c, err := lru.New[K, V](size)
	if err != nil {
		panic(fmt.Errorf("lru: %w (size=%d)", err, size))
	}
	return c
}

// NewSimpleLRU returns a new non-thread-safe LRU cache or panics if size <= 0.
// Use the same way as NewLRU when external synchronization is provided
// (e.g. single-goroutine ownership).
func NewSimpleLRU[K comparable, V any](size int) *simplelru.LRU[K, V] {
	c, err := simplelru.NewLRU[K, V](size, nil)
	if err != nil {
		panic(fmt.Errorf("simplelru: %w (size=%d)", err, size))
	}
	return c
}
