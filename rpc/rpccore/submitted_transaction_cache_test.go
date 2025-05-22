package rpccore_test

import (
	"testing"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/require"
)

func TestAdd(t *testing.T) {
	capacity := 5
	entryTTL := time.Second

	t.Run("Inserting to full cache with expired txs", func(t *testing.T) {
		cache := rpccore.NewSubmittedTransactionsCache(capacity, entryTTL)
		txnHashes := make([]felt.Felt, capacity)
		for i := range capacity {
			txnHashes[i] = *new(felt.Felt).SetUint64(uint64(i))
			cache.Add(&txnHashes[i])
		}
		/// Expire all txs in cache.
		time.Sleep(entryTTL)

		txnHash := new(felt.Felt).SetUint64(uint64(capacity + 1))
		cache.Add(txnHash)
		require.True(t, cache.Contains(txnHash))

		for i := range capacity {
			require.False(t, cache.Contains(&txnHashes[i]))
		}
	})

	t.Run("Inserting to full cache with non-expired txs", func(t *testing.T) {
		cache := rpccore.NewSubmittedTransactionsCache(capacity, entryTTL)
		firstTxnHash := &felt.Zero
		cache.Add(firstTxnHash)
		require.True(t, cache.Contains(firstTxnHash))
		// Populates the cache to cap.
		for i := 1; i < capacity; i++ {
			txnHash := new(felt.Felt).SetUint64(uint64(i))
			cache.Add(txnHash)
		}

		txnHash := new(felt.Felt).SetUint64(uint64(capacity))
		// Expected to remove the least-recently-used entry.
		cache.Add(txnHash)
		require.True(t, cache.Contains(txnHash))
		require.False(t, cache.Contains(firstTxnHash))
	})
}

func TestContains(t *testing.T) {
	capacity := 5
	entryTTL := time.Second

	t.Run("Entry not found in cache", func(t *testing.T) {
		cache := rpccore.NewSubmittedTransactionsCache(capacity, entryTTL)
		txnHash := &felt.One
		require.False(t, cache.Contains(txnHash))
	})

	t.Run("Valid entry found in cache", func(t *testing.T) {
		cache := rpccore.NewSubmittedTransactionsCache(capacity, entryTTL)
		txnHash := &felt.One
		cache.Add(txnHash)
		require.True(t, cache.Contains(txnHash))
	})

	t.Run("Entry found in cache but expired", func(t *testing.T) {
		cache := rpccore.NewSubmittedTransactionsCache(capacity, entryTTL)
		txnHash := &felt.One
		cache.Add(txnHash)
		/// Expire cache entry.
		time.Sleep(entryTTL)
		require.False(t, cache.Contains(txnHash))
	})
}

func TestFlush(t *testing.T) {
	capacity := 5
	entryTTL := time.Second

	t.Run("When there is no expired entry", func(t *testing.T) {
		cache := rpccore.NewSubmittedTransactionsCache(capacity, entryTTL)
		txnHash := &felt.One
		cache.Add(txnHash)

		cache.Flush()
		require.True(t, cache.Contains(txnHash))
	})

	t.Run("When there are expired entries", func(t *testing.T) {
		cache := rpccore.NewSubmittedTransactionsCache(capacity, entryTTL)
		txnHash := &felt.One
		cache.Add(txnHash)

		// Expire the entry
		time.Sleep(entryTTL)
		cache.Flush()
		require.False(t, cache.Contains(txnHash))
	})
}
