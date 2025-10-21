package rpcv9_test

import (
	"testing"

	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionDeduplicationCache_BasicOperations(t *testing.T) {
	t.Run("block zero", func(t *testing.T) {
		t.Run("should handle basic operations with block zero", func(t *testing.T) {
			cache := rpcv9.NewSubscriptionDeduplicationCache[string, int]()

			// Test with block 0
			cache.Put(0, "key1", 1)
			require.False(t, cache.ShouldSend(0, "key1", 1))
			require.True(t, cache.ShouldSend(0, "key1", 2))
		})

		t.Run("should prefer empty slots over eviction", func(t *testing.T) {
			cache := rpcv9.NewSubscriptionDeduplicationCache[string, int]()

			cache.Put(0, "a", 1)
			cache.Put(1, "b", 2) // Should use empty slot, not evict block 0
			require.False(t, cache.ShouldSend(0, "a", 1))
		})
	})

	t.Run("block non-zero", func(t *testing.T) {
		cache := rpcv9.NewSubscriptionDeduplicationCache[string, int]()

		require.True(t, cache.ShouldSend(100, "key1", 1))

		cache.Put(100, "key1", 1)
		cache.Put(101, "key2", 2)
		cache.Put(102, "key3", 3)

		require.False(t, cache.ShouldSend(100, "key1", 1))
		require.True(t, cache.ShouldSend(100, "key1", 2))
		require.True(t, cache.ShouldSend(100, "key2", 1))
	})

	t.Run("should update existing value", func(t *testing.T) {
		cache := rpcv9.NewSubscriptionDeduplicationCache[string, int]()

		cache.Put(100, "key1", 1)
		cache.Put(100, "key1", 2)
		require.True(t, cache.ShouldSend(100, "key1", 1))
		require.False(t, cache.ShouldSend(100, "key1", 2))
	})
}

func TestSubscriptionDeduplicationCache_Eviction(t *testing.T) {
	t.Run("should evict lowest block number", func(t *testing.T) {
		cache := rpcv9.NewSubscriptionDeduplicationCache[string, int]()

		// Fill cache to capacity
		cache.Put(100, "key1", 1)
		cache.Put(101, "key2", 2)
		cache.Put(102, "key3", 3)

		// Add 4th block - should evict block 100 (lowest)
		cache.Put(103, "key4", 4)
		require.True(t, cache.ShouldSend(100, "key1", 1))
		require.False(t, cache.ShouldSend(101, "key2", 2))
		require.False(t, cache.ShouldSend(102, "key3", 3))
		require.False(t, cache.ShouldSend(103, "key4", 4))

		// insert much lower block number - should evict 101 (current lowest)
		cache.Put(50, "d", 4)
		require.True(t, cache.ShouldSend(101, "key2", 2))
		require.False(t, cache.ShouldSend(50, "d", 4))
	})

	t.Run("should not leak keys across eviction", func(t *testing.T) {
		cache := rpcv9.NewSubscriptionDeduplicationCache[string, int]()

		cache.Put(100, "leak", 42)
		cache.Put(101, "k2", 2)
		cache.Put(102, "k3", 3)

		cache.Put(103, "k4", 4)

		require.True(t, cache.ShouldSend(103, "leak", 42))
	})
}

func BenchmarkSubscriptionDeduplicationCache_Put(b *testing.B) {
	cache := rpcv9.NewSubscriptionDeduplicationCache[string, int]()

	b.ResetTimer()
	for i := range b.N {
		cache.Put(uint64(i%10), "key", i)
	}
}

func BenchmarkSubscriptionDeduplicationCache_ShouldSend(b *testing.B) {
	cache := rpcv9.NewSubscriptionDeduplicationCache[string, int]()

	// Pre-populate cache
	for i := range 3 {
		cache.Put(uint64(i), "key", i)
	}

	b.ResetTimer()
	for i := range b.N {
		cache.ShouldSend(uint64(i%3), "key", i)
	}
}

func BenchmarkSubscriptionDeduplicationCache_MixedOperations(b *testing.B) {
	cache := rpcv9.NewSubscriptionDeduplicationCache[string, int]()
	for i := range 3 {
		cache.Put(uint64(i), "k", i)
	}

	b.ResetTimer()
	for i := range b.N {
		if (i & 1) == 0 {
			cache.Put(uint64(i%16), "k", i)
		} else {
			cache.ShouldSend(uint64(i%16), "k", i)
		}
	}
}
