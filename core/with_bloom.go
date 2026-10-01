package core

import (
	"github.com/bits-and-blooms/bloom/v3"
)

// WithBloom pairs a value with the block's event bloom filter.
type WithBloom[T any] struct {
	Value T
	Bloom *bloom.BloomFilter
}
