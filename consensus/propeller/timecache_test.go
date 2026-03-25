package propeller

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeCache_AddAndContains(t *testing.T) {
	cache := NewTimeCache[string](10 * time.Second)

	assert.False(t, cache.Contains("a"), "empty cache should not contain any key")

	cache.Add("a")
	assert.True(t, cache.Contains("a"), "key should be present after Add")
	assert.False(t, cache.Contains("b"), "unrelated key should not be present")
}

func TestTimeCache_Expiration(t *testing.T) {
	// Use a controllable clock so we don't need real sleeps.
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := NewTimeCache[string](5 * time.Second)
	cache.nowFn = func() time.Time { return now }

	cache.Add("x")
	assert.True(t, cache.Contains("x"))

	// Advance time to just before expiry.
	now = now.Add(4 * time.Second)
	assert.True(t, cache.Contains("x"), "should still be present before TTL")

	// Advance time to exactly the expiry moment.
	now = now.Add(1 * time.Second)
	assert.False(t, cache.Contains("x"), "should be expired at TTL boundary")

	// Advance well past expiry.
	now = now.Add(10 * time.Second)
	assert.False(t, cache.Contains("x"), "should be expired well after TTL")
}

func TestTimeCache_RefreshExpiry(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := NewTimeCache[string](5 * time.Second)
	cache.nowFn = func() time.Time { return now }

	cache.Add("k")

	// Advance 3 seconds, then re-add to refresh.
	now = now.Add(3 * time.Second)
	cache.Add("k")

	// Advance another 3 seconds -- would be expired without refresh (6s > 5s),
	// but the refresh pushed the deadline to 3s+5s=8s.
	now = now.Add(3 * time.Second)
	assert.True(t, cache.Contains("k"), "re-add should refresh the TTL")

	// Advance past the refreshed expiry.
	now = now.Add(3 * time.Second)
	assert.False(t, cache.Contains("k"), "should expire after refreshed TTL")
}

func TestTimeCache_Cleanup(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := NewTimeCache[int](2 * time.Second)
	cache.nowFn = func() time.Time { return now }

	for i := range 5 {
		cache.Add(i)
	}
	require.Equal(t, 5, cache.Len())

	// Expire all entries.
	now = now.Add(3 * time.Second)

	// They're expired but still in the map until Cleanup.
	assert.Equal(t, 5, cache.Len(), "expired entries linger until Cleanup")
	assert.False(t, cache.Contains(0), "expired entries should not be found")

	cache.Cleanup()
	assert.Equal(t, 0, cache.Len(), "Cleanup should remove all expired entries")
}

func TestTimeCache_CleanupPartial(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	cache := NewTimeCache[string](5 * time.Second)
	cache.nowFn = func() time.Time { return now }

	cache.Add("early")
	now = now.Add(3 * time.Second)
	cache.Add("late")

	// Advance so "early" is expired but "late" is not.
	now = now.Add(3 * time.Second)

	cache.Cleanup()
	assert.Equal(t, 1, cache.Len(), "only the expired entry should be removed")
	assert.False(t, cache.Contains("early"))
	assert.True(t, cache.Contains("late"))
}

func TestTimeCache_ConcurrentAccess(t *testing.T) {
	cache := NewTimeCache[int](1 * time.Second)

	var wg sync.WaitGroup
	// Hammer the cache from multiple goroutines to verify no races.
	for i := range 100 {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			cache.Add(v)
			cache.Contains(v)
			cache.Cleanup()
		}(i)
	}
	wg.Wait()

	// We just care that it didn't panic or race.
	assert.LessOrEqual(t, cache.Len(), 100)
}
