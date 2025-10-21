package rpcv9_test

import (
	"testing"

	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionCache_BasicOperations(t *testing.T) {
	key1 := "key1"
	key2 := "key2"
	key3 := "key3"
	val1 := 1
	val2 := 2
	val3 := 3
	t.Run("block zero", func(t *testing.T) {
		t.Run("should handle basic operations with block zero", func(t *testing.T) {
			cache := rpcv9.NewSubscriptionCache[string, int]()

			// Test with block 0
			cache.Put(0, &key1, &val1)
			require.False(t, cache.ShouldSend(0, &key1, &val1))
			require.True(t, cache.ShouldSend(0, &key1, &val2))
		})

		t.Run("should prefer empty slots over eviction", func(t *testing.T) {
			cache := rpcv9.NewSubscriptionCache[string, int]()

			cache.Put(0, &key1, &val1)
			cache.Put(1, &key1, &val2) // Should use empty slot, not evict block 0
			require.False(t, cache.ShouldSend(0, &key1, &val1))
		})
	})

	t.Run("block non-zero", func(t *testing.T) {
		cache := rpcv9.NewSubscriptionCache[string, int]()

		require.True(t, cache.ShouldSend(100, &key1, &val1))

		cache.Put(100, &key1, &val1)
		cache.Put(101, &key2, &val2)
		cache.Put(102, &key3, &val3)

		require.False(t, cache.ShouldSend(100, &key1, &val1))
		require.True(t, cache.ShouldSend(100, &key1, &val2))
		require.True(t, cache.ShouldSend(100, &key2, &val1))
	})

	t.Run("should update existing value", func(t *testing.T) {
		cache := rpcv9.NewSubscriptionCache[string, int]()

		cache.Put(100, &key1, &val1)
		cache.Put(100, &key1, &val2)
		require.True(t, cache.ShouldSend(100, &key1, &val1))
		require.False(t, cache.ShouldSend(100, &key1, &val2))
	})
}

func TestSubscriptionCache_Eviction(t *testing.T) {
	key1 := "key1"
	key2 := "key2"
	key3 := "key3"
	key4 := "key4"
	key5 := "key5"
	val1 := 1
	val2 := 2
	val3 := 3
	val4 := 4
	t.Run("should evict lowest block number", func(t *testing.T) {
		cache := rpcv9.NewSubscriptionCache[string, int]()

		// Fill cache to capacity
		cache.Put(100, &key1, &val1)
		cache.Put(101, &key2, &val2)
		cache.Put(102, &key3, &val3)

		// Add 4th block - should evict block 100 (lowest)
		cache.Put(103, &key4, &val4)
		require.True(t, cache.ShouldSend(100, &key1, &val1))
		require.False(t, cache.ShouldSend(101, &key2, &val2))
		require.False(t, cache.ShouldSend(102, &key3, &val3))
		require.False(t, cache.ShouldSend(103, &key4, &val4))

		// insert much lower block number - should evict 101 (current lowest)
		cache.Put(50, &key5, &val4)
		require.True(t, cache.ShouldSend(101, &key2, &val2))
		require.False(t, cache.ShouldSend(50, &key5, &val4))
	})

	t.Run("should not leak keys across eviction", func(t *testing.T) {
		cache := rpcv9.NewSubscriptionCache[string, int]()

		keyLeak := "leak"
		val42 := 42

		cache.Put(100, &keyLeak, &val42)
		cache.Put(101, &key2, &val2)
		cache.Put(102, &key3, &val3)

		cache.Put(103, &key4, &val4)

		require.True(t, cache.ShouldSend(103, &keyLeak, &val42))
	})
}

func BenchmarkSubscriptionCache_Put(b *testing.B) {
	cache := rpcv9.NewSubscriptionCache[string, int]()

	key := "key"
	b.ResetTimer()
	for i := range b.N {
		cache.Put(uint64(i), &key, &i)
	}
}

func BenchmarkSubscriptionCache_ShouldSend(b *testing.B) {
	cache := rpcv9.NewSubscriptionCache[string, int]()

	key := "key"
	// Pre-populate cache
	for i := range 3 {
		cache.Put(uint64(i), &key, &i)
	}

	b.ResetTimer()
	for i := range b.N {
		cache.ShouldSend(uint64(i%3), &key, &i)
	}
}

func BenchmarkSubscriptionCache_MixedOperations(b *testing.B) {
	cache := rpcv9.NewSubscriptionCache[string, int]()
	key := "k"
	for i := range 3 {
		cache.Put(uint64(i), &key, &i)
	}

	b.ResetTimer()
	for i := range b.N {
		if (i & 1) == 0 {
			cache.Put(uint64(i%16), &key, &i)
		} else {
			cache.ShouldSend(uint64(i%16), &key, &i)
		}
	}
}
