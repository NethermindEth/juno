package core

import (
	"github.com/bits-and-blooms/bloom/v3"
)

// WithBloom pairs a value flowing through a subscription feed with the block's
// event bloom filter. A single wrapper is broadcast (by pointer) to all
// subscribers, so the bloom is built once by the producer and shared; only the
// event subscription reads Bloom, the other subscriber kinds ignore it.
type WithBloom[T any] struct {
	Value T
	Bloom *bloom.BloomFilter
}
