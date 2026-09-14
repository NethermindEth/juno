package tracecache

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheSingleOwnerAndPublication(t *testing.T) {
	cache := New[string, string](1)
	key := "key"
	owner := cache.lookupOrStart(&key, nil)
	key = "other" // The lease retains its own key.
	require.Equal(t, cacheLoad, owner.kind)

	const count = 16
	lookups := make([]cacheLookup[string, string], count)
	var group sync.WaitGroup
	for index := range count {
		group.Go(func() { lookups[index] = cache.lookupOrStart(new("key"), nil) })
	}
	group.Wait()
	for _, lookup := range lookups {
		require.Equal(t, cacheWait, lookup.kind)
		require.True(t, owner.work.flight == lookup.done)
	}
	owner.work.Publish("value")
	for _, lookup := range lookups {
		select {
		case <-lookup.done:
		default:
			t.Fatal("publication must release waiters")
		}
	}
	hit := cache.lookupOrStart(new("key"), nil)
	require.Equal(t, cacheHit, hit.kind)
	require.Equal(t, "value", hit.value)
}

func TestCacheReplacementPreservesAcceptedValue(t *testing.T) {
	cache := New[string, string](1)
	cache.lookupOrStart(new("key"), nil).work.Publish("old")
	wantsNew := func(value string) bool { return value == "new" }
	owner := cache.lookupOrStart(new("key"), wantsNew)
	require.Equal(t, cacheLoad, owner.kind)
	require.Equal(t, "old", owner.value)
	hit := cache.lookupOrStart(new("key"), nil)
	require.Equal(t, cacheHit, hit.kind)
	require.Equal(t, "old", hit.value)
	waiting := cache.lookupOrStart(new("key"), wantsNew)
	require.Equal(t, cacheWait, waiting.kind)
	owner.work.Abort()
	select {
	case <-waiting.done:
	default:
		t.Fatal("abort must release waiters")
	}
	require.Equal(t, "old", cache.lookupOrStart(new("key"), nil).value)

	next := cache.lookupOrStart(new("key"), wantsNew)
	require.Equal(t, cacheLoad, next.kind)
	// A released owner cannot overwrite the value or release its successor's waiters.
	owner.work.Publish("stale")
	owner.work.Abort()
	require.Equal(t, "old", cache.lookupOrStart(new("key"), nil).value)
	select {
	case <-next.work.flight:
		t.Fatal("stale owner released its successor")
	default:
	}
	next.work.Publish("new")
	next.work.Abort() // The normal deferred abort after publication is harmless.
	require.Equal(t, "new", cache.lookupOrStart(new("key"), wantsNew).value)
}

func TestCacheEvictionDoesNotReleaseOwner(t *testing.T) {
	cache := New[string, string](1)
	cache.lookupOrStart(new("key"), nil).work.Publish("old")
	owner := cache.lookupOrStart(new("key"), func(string) bool { return false })
	cache.lookupOrStart(new("other"), nil).work.Publish("other value")
	require.Equal(t, "old", owner.value, "the caller retains the evicted value")
	waiting := cache.lookupOrStart(new("key"), nil)
	require.Equal(t, cacheWait, waiting.kind)
	owner.work.Publish("new")
	require.Equal(t, "new", cache.lookupOrStart(new("key"), nil).value)
	select {
	case <-waiting.done:
	default:
		t.Fatal("publication after eviction must release waiters")
	}
}

func TestCacheInstancesAndKeysAreIndependent(t *testing.T) {
	type key struct {
		revision    uint64
		transaction string
	}
	first, second := New[key, *int](2), New[key, *int](2)
	original := key{revision: 1, transaction: "tx"}
	revised := key{revision: 2, transaction: "tx"}
	// Even a zero value is a valid published entry; presence is independent of value.
	first.lookupOrStart(&original, nil).work.Publish(nil)
	hit := first.lookupOrStart(&original, nil)
	require.Equal(t, cacheHit, hit.kind)
	require.Nil(t, hit.value)
	for _, lookup := range []cacheLookup[key, *int]{
		first.lookupOrStart(&revised, nil), second.lookupOrStart(&original, nil),
	} {
		require.Equal(t, cacheLoad, lookup.kind)
		lookup.work.Abort()
	}
}

func TestAcquireCancellation(t *testing.T) {
	cache := New[string, string](1)
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
	cache := New[string, string](1)
	cache.lookupOrStart(new("key"), nil).work.Publish("old")
	wantsNew := func(value string) bool { return value == "new" }
	_, upgrade, err := cache.Acquire(t.Context(), new("key"), wantsNew)
	require.NoError(t, err)
	defer upgrade.Abort()
	waiting := &waitContext{Context: t.Context(), entered: make(chan struct{})}
	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value, work, acquireErr := cache.Acquire(waiting, new("key"), wantsNew)
		if work != nil {
			defer work.Abort()
			work.Publish("new")
		}
		done <- result{value: value, err: acquireErr}
	}()
	<-waiting.entered
	upgrade.Abort()
	outcome := <-done
	require.NoError(t, outcome.err)
	require.Equal(t, "old", outcome.value)
	value, lease, err := cache.Acquire(t.Context(), new("key"), nil)
	require.NoError(t, err)
	require.Nil(t, lease)
	require.Equal(t, "new", value)
}

type waitContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *waitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}
