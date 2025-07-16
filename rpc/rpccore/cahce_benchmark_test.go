package rpccore_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

func BenchmarkCacheAltCopy(b *testing.B) {
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

func BenchmarkCacheOriginalCopy(b *testing.B) {
	const (
		totalEntries = 10000
		numTicks     = 10
	)

	cache := rpccore.NewSubmittedTransactionsCache(totalEntries, 5*time.Second)
	perTick := totalEntries / numTicks

	// prepare keys once
	keys := make([]*felt.Felt, totalEntries)
	for i := 0; i < totalEntries; i++ {
		keys[i] = new(felt.Felt).SetUint64(rand.Uint64())
	}
	keyID := 0

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
