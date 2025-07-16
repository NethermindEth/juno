package rpccore_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	fakeClock := make(chan time.Time, 1)
	cache := rpccore.NewSubmittedTransactionsCacheAlt(fakeClock)
	defer cache.Stop()

	k := new(felt.Felt).SetUint64(123)
	require.False(t, cache.Contains(k))

	cache.Set(k)
	require.True(t, cache.Contains(k))

	// Perform `N-1` ticks
	for i := 0; i < rpccore.NumTimeBuckets-1; i++ {
		fmt.Println("i", i, rpccore.NumTimeBuckets-1)
		fakeClock <- time.Now()
		require.True(t, cache.Contains(k))
	}

	// Nth tick to evict
	fakeClock <- time.Now()
	time.Sleep(10 * time.Millisecond) // The reader may access the txn before the evictor cleans it up
	require.False(t, cache.Contains(k))
}

func BenchmarkCacheLoadDistributed(b *testing.B) {
	const (
		totalEntries = 10000
		numTicks     = 10
	)
	// make the fake clock channel big enough to never block
	fakeClock := make(chan time.Time, numTicks)
	cache := rpccore.NewSubmittedTransactionsCacheAlt(fakeClock)
	defer cache.Stop()

	perTick := totalEntries / numTicks

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for t := 0; t < numTicks; t++ {
			base := uint64(t * perTick)
			for i := 0; i < perTick; i++ {
				cache.Set(new(felt.Felt).SetUint64(base + uint64(i)))
			}
			b.StopTimer()
			fakeClock <- time.Now()
			b.StartTimer()
		}
	}
}
