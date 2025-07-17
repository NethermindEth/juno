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

	fakeClock := make(chan time.Time, 1)
	cache := rpccore.RunTxnCacheWithTicker(t.Context(), fakeClock)

	// Insert nKeys distinct entries
	for i := 0; i < nKeys; i++ {
		k := *new(felt.Felt).SetUint64(uint64(i))
		cache.Set(k)
	}

	// Verify all keys are present
	for i := 0; i < nKeys; i++ {
		k := *new(felt.Felt).SetUint64(uint64(i))
		require.Truef(t, cache.Contains(k),
			"expected key %d to be present after bulk insert", i)
	}

	// Evict all entries
	for i := 0; i < rpccore.NumTimeBuckets; i++ {
		fakeClock <- time.Now()
	}

	for i := 0; i < nKeys; i++ {
		k := *new(felt.Felt).SetUint64(uint64(i))
		require.Falsef(t, cache.Contains(k),
			"expected key %d to be present after bulk insert", i)
	}
}

// Todo: there is a race. It is in the situaion where we call Contains and
// the evict loop at the same time. The contains might read the time before/after
// evict updates it.
//
// Evict calls. Time updates. Contains calls. All good. No longer present.
// Evict calls. Contain calls before the time gets updated. Seems to be present/
// Eg query at 4s. Always gets correct result.
// Eg querys at 6s. Always gets correct result.
// Eg querys at 5s. Result depends on whether the evictor updates the timeCount before Cotains.
func TestCache_RaceCondition(t *testing.T) {
	for run := 0; run < 10; run++ {
		t.Run(fmt.Sprintf("run-%d", run), func(t *testing.T) {
			fakeClock := make(chan time.Time, 1)
			cache := rpccore.RunTxnCacheWithTicker(t.Context(), fakeClock)

			k := *new(felt.Felt).SetUint64(123)
			require.False(t, cache.Contains(k), "initial Contains should be false")

			cache.Set(k)
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
//
// Key on felt.Felt. Update evict to clean in place.Update Benchmark to preallcoate the keys. b.Start()/Stop() arroundfakeTimer.
// Run 1: BenchmarkCacheLoadDistributed-24    	    1604	   1006585 ns/op	    1049 B/op	       3 allocs/op
// Run 2: BenchmarkCacheLoadDistributed-24    	    1808	    899488 ns/op	    1178 B/op	       3 allocs/op
//
// Key on felt.Felt. Update evict to clean in place.Update Benchmark to preallcoate the keys. b.Start()/Stop() arroundfakeTimer.
// Set also calls Contains to prevent duplicates
// Run 1: BenchmarkCacheAlt-24    	 			    487		   	2447542 ns/op	    1115 B/op	       3 allocs/op
// Run 2: BenchmarkCacheAlt-24    	   				471	   		2709032 ns/op	    1150 B/op	       3 allocs/op
//
// Key on felt.Felt. Update evict to clean in place.Update Benchmark to preallcoate the keys. b.Start()/Stop() arroundfakeTimer.
// Set also calls Contains to prevent duplicates. Single lock.
// Run 1: BenchmarkCacheAlt-24    	  				372	   		3138484 ns/op	    3611 B/op	       0 allocs/op
// Run 2: BenchmarkCacheAlt-24    	    			386	   		3009791 ns/op	    3480 B/op	       0 allocs/op

// func BenchmarkCacheAlt(b *testing.B) {
// 	const (
// 		totalEntries = 10000
// 		numTicks     = 10
// 	)
// 	// make the fake clock channel big enough to never block
// 	fakeClock := make(chan time.Time, numTicks)
// 	cache := rpccore.NewSubmittedTransactionsCacheAlt(fakeClock)
// 	defer cache.Stop()

// 	perTick := totalEntries / numTicks

// 	keys := make([]felt.Felt, totalEntries)
// 	for i := 0; i < totalEntries; i++ {
// 		keys[i].SetUint64(rand.Uint64())
// 	}

// 	b.ResetTimer()
// 	for n := 0; n < b.N; n++ {
// 		keyID := 0
// 		for t := 0; t < numTicks; t++ {
// 			// Add all the txns for this round
// 			for i := 0; i < perTick; i++ {
// 				cache.Set(keys[keyID])
// 				keyID++
// 			}
// 			// Trigger the eviction given one tick ("second") has passed
// 			fakeClock <- time.Now()
// 		}
// 	}
// }

// // Benchmark:
// //
// // Just pure add, no time sleep in the benchmark
// // Run 1: BenchmarkCacheOriginal-24    	       1	4778887699 ns/op	1640341336 B/op	   21219 allocs/op
// // Run 2: BenchmarkCacheOriginal-24    	       1	4734790561 ns/op	1640341384 B/op	   21225 allocs/op
// //
// // Sleep in the benchmark to simulate ticks, but do not count towards the benchmark
// // Run1: BenchmarkCacheOriginal-24    	       1	2777534374 ns/op	 995962056 B/op	   21815 allocs/op
// // Run2: BenchmarkCacheOriginal-24    	       1	2786046135 ns/op	 989245880 B/op	   21506 allocs/op

// func BenchmarkCacheOriginal(b *testing.B) {
// 	const (
// 		totalEntries = 10000
// 		numTicks     = 10
// 	)

// 	cache := rpccore.NewSubmittedTransactionsCache(totalEntries, 5*time.Second)
// 	perTick := totalEntries / numTicks

// 	// prepare keys once
// 	keys := make([]*felt.Felt, totalEntries)
// 	for i := 0; i < totalEntries; i++ {
// 		keys[i] = new(felt.Felt).SetUint64(rand.Uint64())
// 	}
// 	keyID := 0

// 	b.ResetTimer()
// 	for n := 0; n < b.N; n++ {
// 		for t := 0; t < numTicks; t++ {
// 			// Add all the txns for this second
// 			for i := 0; i < perTick; i++ {
// 				cache.Add(keys[keyID])
// 				keyID++
// 			}
// 			// simulate one second passing per tick, but don't count it in the benchmark
// 			b.StopTimer()
// 			time.Sleep(1 * time.Second)
// 			b.StartTimer()
// 		}
// 	}
// }

// Benchmark comparison
//
// Both benchmarks involve adding 10k txns evenly into the cache over 10s. The TTL in both is 5s, so both benchmarks
// should simulate the entire lifecyle of both implementations.
//
// Original:  	BenchmarkCacheOriginal-24    	   		    1			2786046135 ns/op	989245880 B/op	    21506 allocs/op
// New:			BenchmarkCacheAlt-24    	 			    487		   	2447542 ns/op	    1115 B/op	        3 allocs/op
// Reduction:                                 							91%  				99%					98.5%
//
// Summary:
//   The ns/op goes from 2,786ms to 2ms.
//   The B/op does from 989245880 B/op to 1115 B/op. Todo: this seems extremely high...investigate
