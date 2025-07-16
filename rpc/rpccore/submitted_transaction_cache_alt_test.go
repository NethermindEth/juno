package rpccore_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

func TestSetGet(t *testing.T) {
	tick := 20 * time.Millisecond
	cache := rpccore.NewSubmittedTransactionsCacheAlt(tick)
	defer cache.Stop()

	key := new(felt.Felt).SetUint64(42)
	now := time.Now()
	cache.Set(key, now)

	ts, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected key to be present immediately after Set")
	}
	if !ts.Equal(now) {
		t.Errorf("expected timestamp %v, got %v", now, ts)
	}
}

func TestEviction(t *testing.T) {
	tick := 10 * time.Millisecond
	cache := rpccore.NewSubmittedTransactionsCacheAlt(tick)
	defer cache.Stop()

	key := new(felt.Felt).SetUint64(42)
	cache.Set(key, time.Now())

	if _, ok := cache.Get(key); !ok {
		t.Fatal("expected key to be present immediately")
	}

	// before TTL (5*tick)
	time.Sleep(3 * tick)
	if _, ok := cache.Get(key); !ok {
		t.Fatal("expected key still present before TTL elapsed")
	}

	// after TTL
	time.Sleep(3 * tick)
	if _, ok := cache.Get(key); ok {
		t.Fatal("expected key to be evicted after TTL")
	}
}

func TestMultipleKeysDifferentBuckets(t *testing.T) {
	tick := 100 * time.Millisecond
	cache := rpccore.NewSubmittedTransactionsCacheAlt(tick)
	defer cache.Stop()

	key1 := new(felt.Felt).SetUint64(312)
	cache.Set(key1, time.Now())
	time.Sleep(2 * tick)

	key2 := new(felt.Felt).SetUint64(24)
	cache.Set(key2, time.Now())

	// At this point:
	//   • key1 is in bucket 0, and will be evicted on the 5th tick
	//   • key2 is in bucket 2, and will be evicted on the 7th tick
	//
	// Sleep 3 more ticks (so total elapsed ~5 ticks ):
	time.Sleep(3 * tick)

	if _, ok := cache.Get(key1); ok {
		t.Error("expected key1 to be evicted after 5 ticks, but it is still present")
	}
	if _, ok := cache.Get(key2); !ok {
		t.Error("expected key2 to still be present after 5 ticks, but it was evicted too early")
	}
}

func TestStopPreventsFurtherEviction(t *testing.T) {
	tick := 10 * time.Millisecond
	cache := rpccore.NewSubmittedTransactionsCacheAlt(tick)

	key := new(felt.Felt).SetUint64(42)
	cache.Set(key, time.Now())
	cache.Stop()

	time.Sleep(6 * tick)
	if _, ok := cache.Get(key); !ok {
		t.Error("expected key 3 to remain after Stop(), but it was evicted")
	}
}

// BenchmarkLoadDistributed measures inserting 10 000 entries evenly over 10 ticks.
func BenchmarkCacheLoadDistributed(b *testing.B) {
	const (
		totalEntries = 100
		numTicks     = 10
	)
	tick := 10 * time.Millisecond

	for n := 0; n < b.N; n++ {
		cache := rpccore.NewSubmittedTransactionsCacheAlt(tick)

		perTick := totalEntries / numTicks

		b.ResetTimer()
		for t := 0; t < numTicks; t++ {
			startVal := uint64(t * perTick)
			for i := 0; i < perTick; i++ {
				key := new(felt.Felt).SetUint64(startVal + uint64(i))
				cache.Set(key, time.Now())
			}
			b.StopTimer()
			time.Sleep(tick)
			b.StartTimer()
		}
		b.StopTimer()

		cache.Stop()
	}
}
