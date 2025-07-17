package rpccore_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

// Summary:
// BenchmarkCacheOriginal-24    	       1		2776591293 ns/op	    989802040 B/op	   21610 allocs/op
// BenchmarkCacheAlt-24    	     		   504	   	   3103484 ns/op	    	    1 B/op	       0 allocs/op
//
// In a nutshell
//   Old LRU approach: flush → allocate a giant slice → scan 5 000 entries → delete expired → repeat 10 000× → GBs of allocations.
//				Reads, Writes and Deletes all use the same locks.
//				Every read alloctes a new O(n) slice (inside flush() Keys()).
//				Allocates a map (key-felt-pointer, value-time.Time) and list, both of which need maintained etc.
//
//
//   Time‑wheel approach: constant‑time ring buffer → in‑place deletes → no allocations on the hot path → zero allocs/op and < 5 ms latency.
//				Reads, Writes are completely seperated from Deletes.
//				Delete in-place.
//				Reads and Writes use seperate locks.
//				Memory is reused, so we don't allocate new memory (assuming that we never go above 1024 TPS).
//				Allocates [6]map (key-felt-valuetype, value-struct{}). No list. No time.Time.
//
//
// Longer TLDR:
// Why is the new approach so much more efficient?
// There are a few reasons why the old approach was inefficient. The most signifcaint is probably that we call flush()
// on every Add(). Under the hood, flush() creates a new array and inserts all the map Keys into it. This is very expensive,
// and occurs on every Add! Profiling suggests we end up using 1-2GB worth of memory because of this.
// 5000 entries exist in the map over the 5s TTL. 5000 x 32bytes = 160,000 bytes.
// But we make 10,000 calls to Add() and therefore flush(), meaning we get 10,000*160,000 bytes = 1.6 GB from this alone!
// This drives up allocs/op, B/op and reduces the latency.
//
// Another significant driver of the ns/op is the lock contention. Every Add() and Contains() causes a lock contention, which
// is suboptimal and unnessecary.
//
// The new approach follows very different logic, so we avoid this problem completely.
// In the new approach we allocate memory up front, and reuse it throughout the course of the program. This
// helps drive down allocs/op, B/op and reduces ns/op.
//
// The lock mechanism is also quite different. Since we bucket entries by time, it means that entries that belong to
// different time buckets can't block each other. We can further reduce lock contention by sharding the locks
// within a time bucket. Further, the reads and writes within a time bucket should not conflict, since we use RW locks.
// Finally, if the TTL is 5s, we have 6 time buckets. On the 6th second, we allocate the new entries into the 6th bucket,
// and can freely remove the entries in the 1st bucket without conflicting with any other time bucket, further reducing
// lock contention.

// 1 lock per time bucket
// BenchmarkCacheAlt-24    	     676	   1886141 ns/op	       0 B/op	       0 allocs/op
// BenchmarkCacheAlt-24    	     702	   1952262 ns/op	       0 B/op	       0 allocs/op

//
// 2 locks per time bucket
// BenchmarkCacheAlt-24    	     522	   2099966 ns/op	       0 B/op	       0 allocs/op
// BenchmarkCacheAlt-24    	     549	   2187128 ns/op	       0 B/op	       0 allocs/op

// 8 locks per time bucket
// BenchmarkCacheAlt-24    	     522	   2614859 ns/op	       0 B/op	       0 allocs/op
// BenchmarkCacheAlt-24    	     478	   3087044 ns/op	       0 B/op	       0 allocs/op

// 64 locks per time bucket
// BenchmarkCacheAlt-24    	     206	   5942285 ns/op	       0 B/op	       0 allocs/op
// BenchmarkCacheAlt-24    	     199	   5707581 ns/op	       0 B/op	       0 allocs/op

// 256 locks per time bucket
// BenchmarkCacheAlt-24    	     100	  19890581 ns/op	       0 B/op	       0 allocs/op
// BenchmarkCacheAlt-24    	     100	  19662352 ns/op	       0 B/op	       0 allocs/op
//
// Locks do reduce the lock contention in the set flow. But the evict loop needs to acquire more locks
// and incur the overhead of locking each of those. It seems that one lock per bucket is more performant.
//
// Without multiple locks per tick bucket
// BenchmarkCacheAlt-24    	     852	   1605398 ns/op	       0 B/op	       0 allocs/op
// BenchmarkCacheAlt-24    	     742	   1495547 ns/op	       0 B/op	       0 allocs/op

func BenchmarkCacheAlt(b *testing.B) {
	const (
		totalEntries = 10000
		numTicks     = 10
	)
	// make the fake clock channel big enough to never block
	fakeClock := make(chan time.Time, numTicks)
	cache := rpccore.NewSubmittedTransactionsCacheAlt(fakeClock)
	defer cache.Stop()

	perTick := totalEntries / numTicks

	keys := make([]felt.Felt, totalEntries)
	for i := 0; i < totalEntries; i++ {
		keys[i].SetUint64(rand.Uint64())
	}

	// Only interested in the performance of the cache as its in use, after it's all set up.
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		keyID := 0
		for t := 0; t < numTicks; t++ {
			// Add all the txns for this round
			for i := 0; i < perTick; i++ {
				cache.Set(keys[keyID])
				keyID++
			}
			// Trigger the eviction given one tick ("second") has passed
			fakeClock <- time.Now()
		}
	}
}

func BenchmarkCacheOriginal(b *testing.B) {
	const (
		totalEntries = 10000
		numTicks     = 10
	)

	cache := rpccore.NewSubmittedTransactionsCache(totalEntries, 5*time.Second)
	perTick := totalEntries / numTicks

	keys := make([]*felt.Felt, totalEntries)
	for i := 0; i < totalEntries; i++ {
		keys[i] = new(felt.Felt).SetUint64(rand.Uint64())
	}
	keyID := 0

	// Only interested in the performance of the cache as its in use, after it's all set up.
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for t := 0; t < numTicks; t++ {
			// Add all the txns for this second
			for i := 0; i < perTick; i++ {
				cache.Add(keys[keyID])
				keyID++
				if keyID == totalEntries {
					keyID = 0
				}
			}
			// simulate one second passing per tick, but don't count it in the benchmark
			b.StopTimer()
			time.Sleep(1 * time.Second)
			b.StartTimer()
		}
	}
}

func BenchmarkCacheSyncMap(b *testing.B) {
	const (
		totalEntries = 10000
		numTicks     = 10
	)
	// fake clock channel: buffered so sends never block
	fakeClock := make(chan time.Time, numTicks)
	cache := rpccore.NewSubmittedTransactionsCacheSyncMap(fakeClock)
	defer cache.Stop()

	perTick := totalEntries / numTicks

	// pre‑allocate keys once
	keys := make([]felt.Felt, totalEntries)
	for i := 0; i < totalEntries; i++ {
		keys[i].SetUint64(rand.Uint64())
	}
	keyID := 0

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for t := 0; t < numTicks; t++ {
			// insert perTick entries
			for i := 0; i < perTick; i++ {
				cache.Set(keys[keyID])
				keyID++
				if keyID == totalEntries {
					keyID = 0
				}
			}
			// drive one eviction tick without counting it
			b.StopTimer()
			fakeClock <- time.Now()
			b.StartTimer()
		}
	}
}
