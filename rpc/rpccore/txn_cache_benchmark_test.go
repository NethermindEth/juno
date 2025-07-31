package rpccore_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/require"
)

func BenchmarkCache(b *testing.B) {
	const (
		totalEntries = 10000
		numTicks     = 10
		ttl          = 1 // Only affects Contains(), effectively marking all entries as extinct. Irrelvant here.
	)

	cache := rpccore.NewTxnCache(ttl, totalEntries)
	fakeClock := make(chan time.Time, 1)
	cache.WithTicker(fakeClock)
	go func() {
		err := cache.Run(b.Context())
		require.NoError(b, err)
	}()

	perTick := totalEntries / numTicks

	keys := make([]felt.Felt, totalEntries)
	for i := range totalEntries {
		keys[i].SetUint64(rand.Uint64())
	}

	// Not interested in the one time setup cost of the cache
	b.ResetTimer()
	for range b.N {
		keyID := 0
		for range numTicks {
			// Add all the txns for this round
			for range perTick {
				cache.Add(keys[keyID])
				keyID++
			}
			// Trigger the eviction given one tick has passed
			fakeClock <- time.Now()
		}
	}
}
