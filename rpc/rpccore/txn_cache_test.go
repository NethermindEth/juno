package rpccore_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/require"
)

func TestBulkSetContains(t *testing.T) {
	const nKeys = 2048

	cache := rpccore.NewTxnCache()
	fakeClock := make(chan time.Time, 1)
	cache.WithTicker(fakeClock)
	go func() {
		err := cache.Run(t.Context())
		require.NoError(t, err)
	}()

	// Insert nKeys distinct entries
	for i := range nKeys {
		k := *new(felt.Felt).SetUint64(uint64(i))
		cache.Add(k)
	}

	// Verify all keys are present
	for i := range nKeys {
		k := *new(felt.Felt).SetUint64(uint64(i))
		require.Truef(t, cache.Contains(k),
			"expected key %d to be present after bulk insert", i)
	}

	// Evict all entries
	for range rpccore.NumTimeBuckets {
		fakeClock <- time.Now()
	}

	for i := range nKeys {
		k := *new(felt.Felt).SetUint64(uint64(i))
		require.Falsef(t, cache.Contains(k),
			"expected key %d to be present after bulk insert", i)
	}
}

func TestCache_RaceCondition(t *testing.T) {
	// Run multiple tests to help detect potential race conditions
	for run := range 10 {
		t.Run(fmt.Sprintf("run-%d", run), func(t *testing.T) {
			cache := rpccore.NewTxnCache()
			fakeClock := make(chan time.Time, 1)
			cache.WithTicker(fakeClock)
			go func() {
				err := cache.Run(t.Context())
				require.NoError(t, err)
			}()

			k := *new(felt.Felt).SetUint64(123)
			require.False(t, cache.Contains(k), "initial Contains should be false")

			cache.Add(k)
			require.True(t, cache.Contains(k), "Contains immediately after Set should be true")

			// 4 seconds pass by. Contains must return true each time.
			for second := 1; second < 5; second++ {
				fakeClock <- time.Now()
				require.True(t, cache.Contains(k))
			}

			// On the 5th second, Contains() should return false.
			fakeClock <- time.Now()
			time.Sleep(1 * time.Millisecond) // Contains may return incorrect result if it races with the tick update.
			require.False(t, cache.Contains(k), "after full TTL, key must be evicted")
		})
	}
}
