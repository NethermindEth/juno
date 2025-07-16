package rpccore_test

import (
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

	k := *new(felt.Felt).SetUint64(123)
	require.False(t, cache.Contains(k))

	cache.Set(k)
	require.True(t, cache.Contains(k))

	// Perform `N-1` ticks
	for i := 0; i < rpccore.NumTimeBuckets-1; i++ {
		fakeClock <- time.Now()
		require.True(t, cache.Contains(k))
	}

	// Nth tick to evict
	fakeClock <- time.Now()
	time.Sleep(10 * time.Millisecond) // The reader may access the txn before the evictor cleans it up
	require.False(t, cache.Contains(k))
}

// Benchmark:
//
// numShards=256, NumBuckets= 6
//
// Key on *felt.Felt
// Run 1: BenchmarkCacheLoadDistributed-24    	     751	   1725330 ns/op	  802390 B/op	   15052 allocs/op
// Run 2: BenchmarkCacheLoadDistributed-24    	     936	   1660684 ns/op	  798422 B/op	   15025 allocs/op
//
// Key on felt.Felt. Note: we have signifcaintly fewer allocs.
// Run 1: BenchmarkCacheLoadDistributed-24    	     750	   1680157 ns/op	 1008797 B/op	    5070 allocs/op
// Run 2: BenchmarkCacheLoadDistributed-24    	     764	   1569821 ns/op	  984177 B/op	    4999 allocs/op
//
// Key on felt.Felt. Update evict to clean in place
// Run 1: BenchmarkCacheLoadDistributed-24    	    1165	   1076660 ns/op	    1209 B/op	       4 allocs/op
// Run 2: BenchmarkCacheLoadDistributed-24    	    1189	   1267186 ns/op	     951 B/op	       3 allocs/op

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
				cache.Set(*new(felt.Felt).SetUint64(base + uint64(i)))
			}
			b.StopTimer()
			fakeClock <- time.Now()
			b.StartTimer()
		}
	}
}
