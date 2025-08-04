package rpccore_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/require"
)

func TestBulkSetContains(t *testing.T) {
	const nKeys = 2048

	const ttl = time.Second

	cache := rpccore.NewTransactionCache(ttl, 1024)
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
		require.Truef(
			t,
			cache.Contains(k),
			"expected key %d to be present after bulk insert",
			i)
	}

	// Wait for the entries to expire
	time.Sleep(ttl)

	// Verify all the entries are expired
	for i := range nKeys {
		k := *new(felt.Felt).SetUint64(uint64(i))
		require.Falsef(
			t,
			cache.Contains(k),
			"expected key %d to be deleted after ttl was hit",
			i)
	}

	// Entries don't come back to life etc as the cache buckets rotate
	time.Sleep(ttl)
	for i := range nKeys {
		k := *new(felt.Felt).SetUint64(uint64(i))
		require.Falsef(
			t,
			cache.Contains(k),
			"expected key %d to be deleted after ttl was hit",
			i)
	}
	time.Sleep(ttl)
	for i := range nKeys {
		k := *new(felt.Felt).SetUint64(uint64(i))
		require.Falsef(
			t,
			cache.Contains(k),
			"expected key %d to be deleted after ttl was hit",
			i)
	}
}
