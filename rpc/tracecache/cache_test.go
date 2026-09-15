package tracecache_test

import (
	"context"
	"testing"
	"testing/synctest"

	"github.com/NethermindEth/juno/rpc/tracecache"
	"github.com/stretchr/testify/require"
)

func TestCacheSingleOwnerAndPublication(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := tracecache.New[string, string](1)
		key := "key"
		value, owner, err := cache.Acquire(t.Context(), &key, nil)
		require.NoError(t, err)
		require.NotNil(t, owner)
		defer owner.Abort()
		require.Empty(t, value)
		key = "other" // The lease retains its own key.

		const count = 16
		waiters := make([]<-chan acquireResult, count)
		for i := range waiters {
			waiters[i] = acquireAsync(t.Context(), cache, new("key"), nil)
		}
		synctest.Wait()
		for _, waiter := range waiters {
			require.Empty(t, waiter, "requests must wait for publication")
		}
		owner.Publish("value")
		for _, waiter := range waiters {
			result := <-waiter
			require.NoError(t, result.err)
			require.Nil(t, result.lease)
			require.Equal(t, "value", result.value)
		}
	})
}

func TestCacheReplacementPreservesAcceptedValue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := tracecache.New[string, string](1)
		key := new("key")
		_, seed, err := cache.Acquire(t.Context(), key, nil)
		require.NoError(t, err)
		seed.Publish("old")
		wantsNew := func(value string) bool { return value == "new" }
		value, owner, err := cache.Acquire(t.Context(), key, wantsNew)
		require.NoError(t, err)
		require.NotNil(t, owner)
		defer owner.Abort()
		require.Equal(t, "old", value)
		value, lease, err := cache.Acquire(t.Context(), key, nil)
		require.NoError(t, err)
		require.Nil(t, lease)
		require.Equal(t, "old", value)

		waiting := acquireAsync(t.Context(), cache, key, wantsNew)
		synctest.Wait()
		require.Empty(t, waiting)
		owner.Abort()
		next := <-waiting
		require.NoError(t, next.err)
		require.NotNil(t, next.lease)
		defer next.lease.Abort()
		require.Equal(t, "old", next.value)

		waiting = acquireAsync(t.Context(), cache, key, wantsNew)
		synctest.Wait()
		require.Empty(t, waiting)
		// A released lease cannot overwrite the value or release its successor's waiters.
		owner.Publish("stale")
		owner.Abort()
		synctest.Wait()
		require.Empty(t, waiting)
		value, lease, err = cache.Acquire(t.Context(), key, nil)
		require.NoError(t, err)
		require.Nil(t, lease)
		require.Equal(t, "old", value)

		next.lease.Publish("new")
		next.lease.Abort() // The normal deferred abort after publication is harmless.
		result := <-waiting
		require.NoError(t, result.err)
		require.Nil(t, result.lease)
		require.Equal(t, "new", result.value)
	})
}

func TestCacheEvictionDoesNotReleaseOwner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := tracecache.New[string, string](1)
		key := new("key")
		_, seed, err := cache.Acquire(t.Context(), key, nil)
		require.NoError(t, err)
		seed.Publish("old")
		value, owner, err := cache.Acquire(t.Context(), key, func(string) bool { return false })
		require.NoError(t, err)
		require.NotNil(t, owner)
		defer owner.Abort()
		_, other, err := cache.Acquire(t.Context(), new("other"), nil)
		require.NoError(t, err)
		other.Publish("other value")
		require.Equal(t, "old", value, "the caller retains the evicted value")

		waiting := acquireAsync(t.Context(), cache, key, nil)
		synctest.Wait()
		require.Empty(t, waiting)
		owner.Publish("new")
		result := <-waiting
		require.NoError(t, result.err)
		require.Nil(t, result.lease)
		require.Equal(t, "new", result.value)
	})
}

func TestCacheInstancesAndKeysAreIndependent(t *testing.T) {
	type key struct {
		revision    uint64
		transaction string
	}
	first, second := tracecache.New[key, *int](2), tracecache.New[key, *int](2)
	original := key{revision: 1, transaction: "tx"}
	revised := key{revision: 2, transaction: "tx"}
	// Even a zero value is a valid published entry; presence is independent of value.
	_, owner, err := first.Acquire(t.Context(), &original, nil)
	require.NoError(t, err)
	owner.Publish(nil)
	value, lease, err := first.Acquire(t.Context(), &original, nil)
	require.NoError(t, err)
	require.Nil(t, lease)
	require.Nil(t, value)
	value, lease, err = first.Acquire(t.Context(), &revised, nil)
	require.NoError(t, err)
	require.NotNil(t, lease)
	defer lease.Abort()
	require.Nil(t, value)
	value, lease, err = second.Acquire(t.Context(), &original, nil)
	require.NoError(t, err)
	require.NotNil(t, lease)
	defer lease.Abort()
	require.Nil(t, value)
}

func TestAcquireCancellation(t *testing.T) {
	cache := tracecache.New[string, string](1)
	_, owner, err := cache.Acquire(t.Context(), new("key"), nil)
	require.NoError(t, err)
	defer owner.Abort()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, lease, err := cache.Acquire(ctx, new("key"), nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, lease)
	owner.Publish("old")
	value, lease, err := cache.Acquire(t.Context(), new("key"), nil)
	require.NoError(t, err)
	require.Nil(t, lease)
	require.Equal(t, "old", value)
}

func TestAcquireRetryAfterAbort(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cache := tracecache.New[string, string](1)
		key := new("key")
		_, seed, err := cache.Acquire(t.Context(), key, nil)
		require.NoError(t, err)
		seed.Publish("old")
		wantsNew := func(value string) bool { return value == "new" }
		_, upgrade, err := cache.Acquire(t.Context(), key, wantsNew)
		require.NoError(t, err)
		defer upgrade.Abort()
		done := make(chan acquireResult, 1)
		go func() {
			value, lease, acquireErr := cache.Acquire(t.Context(), key, wantsNew)
			if lease != nil {
				defer lease.Abort()
				lease.Publish("new")
			}
			done <- acquireResult{value: value, lease: lease, err: acquireErr}
		}()
		synctest.Wait()
		require.Empty(t, done)
		upgrade.Abort()
		outcome := <-done
		require.NoError(t, outcome.err)
		require.NotNil(t, outcome.lease)
		require.Equal(t, "old", outcome.value)
		value, lease, err := cache.Acquire(t.Context(), key, nil)
		require.NoError(t, err)
		require.Nil(t, lease)
		require.Equal(t, "new", value)
	})
}

type acquireResult struct {
	value string
	lease *tracecache.Lease[string, string]
	err   error
}

func acquireAsync(
	ctx context.Context,
	cache *tracecache.Cache[string, string],
	key *string,
	accepts func(string) bool,
) <-chan acquireResult {
	done := make(chan acquireResult, 1)
	go func() {
		value, lease, err := cache.Acquire(ctx, key, accepts)
		done <- acquireResult{value: value, lease: lease, err: err}
	}()
	return done
}
