package rpccore_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/types/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/require"
)

func TestBulkSetContains(t *testing.T) {
	const nKeys = 2048
	const ttl = 10 * time.Second

	cache := rpccore.NewTransactionCache(ttl, 1024)
	fakeClock := make(chan time.Time, 1)
	cache.WithTicker(fakeClock)
	go func() {
		err := cache.Run(t.Context())
		require.NoError(t, err)
	}()

	// Insert nKeys distinct entries
	for i := range nKeys {
		k := new(felt.Felt).SetUint64(uint64(i))
		cache.Add(k)
	}

	assertKeys := func(expected bool, msg string) {
		for i := range nKeys {
			k := *new(felt.Felt).SetUint64(uint64(i))
			require.Equalf(t, expected, cache.Contains(&k), msg, i)
		}
	}

	// Verify all keys are present
	assertKeys(true, "expected key %d to be present after bulk insert")

	// Trigger a tick, rotate buckets
	fakeClock <- time.Now()

	// Keys still valid
	assertKeys(true, "expected key %d to be present after first tick")

	// Trigger another tick
	fakeClock <- time.Now()

	// Allow the evictor to trigger time update
	// In practice the `time.Since(insertTime) <= c.ttl` check in Contains
	// prevents this data race. But we are overriding time here.
	time.Sleep(10 * time.Millisecond)

	// Keys should be evicted
	assertKeys(false, "expected key %d to be deleted after ttl was hit")

	// Keys should remain evicted after further rotation
	fakeClock <- time.Now()
	assertKeys(false, "expected key %d to be deleted after ttl was hit")
}
