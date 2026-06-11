package timecache_test

import (
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/consensus/propeller/timecache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeCacheSequentially(t *testing.T) {
	t.Parallel()
	t.Run("correctly expires keys", func(t *testing.T) {
		t.Parallel()

		const expiry = 3 * time.Second
		tc := timecache.New[int](3, expiry)

		key := 3
		tc.Add(&key)

		require.True(t, tc.Get(&key), "key should exists while it hasn't expired")

		time.Sleep(expiry + 1*time.Second)

		require.False(t, tc.Get(&key), "key shouldn't exist after expiration window")
	})

	t.Run("correctly increases in size the cache's exceeds orignal size", func(t *testing.T) {
		t.Parallel()

		const size = 3
		const expiry = 3 * time.Second

		// Fill the time cache with data to it's maximum size
		tc := timecache.New[int](size, expiry)
		for i := range size {
			tc.Add(&i)
		}
		for i := range size {
			require.Truef(t, tc.Get(&i), "key %d should still exist", i)
		}

		// Add more element and check that old and new keys both exist in the cache
		time.Sleep(time.Second)
		for i := range size {
			newKey := i + size
			tc.Add(&newKey)
			require.Truef(t, tc.Get(&i), "key %d should exist", i)
			require.Truef(t, tc.Get(&newKey), "new key %d should also exist ", newKey)
		}

		// Wait for old keys to expire
		time.Sleep(2*time.Second + 200*time.Millisecond)
		for i := range size {
			newKey := i + size
			require.Falsef(t, tc.Get(&i), "key %d shouldn't exist", i)
			require.Truef(t, tc.Get(&newKey), "new key %d should exist ", newKey)
		}

		// Add back the initial keys and check that they can both are being held
		for i := range size {
			newKey := i + size
			tc.Add(&i)
			require.Truef(t, tc.Get(&i), "key %d should exist again", i)
			require.Truef(t, tc.Get(&newKey), "new key %d should still exist ", newKey)
		}
	})
}

func TestTimeCacheConcurrently(t *testing.T) {
	const size = 100
	const expiry = 1 * time.Second

	tc := timecache.New[int](size, expiry)

	// Go-routine A will send non stop elements on an interval for 3 seconds
	// Go-routine B will check for the elements right after
	// Go-routine C will check for the elements after expiry time

	fastCheckCh := make(chan int)

	type timedInt struct {
		val  int
		time time.Time
	}
	slowCheckCh := make(chan timedInt)

	var wg sync.WaitGroup

	// Go-routine A
	const sendPeriod = expiry * 3
	wg.Go(func() {
		key := 0
		timeout := time.After(sendPeriod)
		for {
			select {
			case <-timeout:
				close(fastCheckCh)
				close(slowCheckCh)
				return
			case <-time.After(time.Duration(rand.IntN(250)+1) * time.Millisecond):
				tc.Add(&key)
				fastCheckCh <- key
				slowCheckCh <- timedInt{val: key, time: time.Now()}
				key += 1
			}
		}
	})

	// Go-routine B
	wg.Go(func() {
		for v := range fastCheckCh {
			assert.Truef(t, tc.Get(&v), "value %d should still exists", v)
		}
	})

	// Go-routine C
	wg.Go(func() {
		valToReview := 0
		valsToCheck := make([]timedInt, 0, 100)
		waitDuration := time.Now().Add(sendPeriod * 2)

		for {
			select {
			case timedInt, ok := <-slowCheckCh:
				if !ok {
					if valToReview == len(valsToCheck) {
						// This shouldn't happen, as with the current config after sending
						// is finished there should be more values to check
						return
					}
					slowCheckCh = nil
					continue
				}
				valsToCheck = append(valsToCheck, timedInt)
				if valToReview == len(valsToCheck)-1 {
					waitDuration = valsToCheck[valToReview].time.Add(expiry)
				}
			case <-time.After(time.Until(waitDuration)):
				val := valsToCheck[valToReview]
				assert.Falsef(t, tc.Get(&val.val), "value %d shouldn't exist", val.val)

				valToReview += 1
				if valToReview == len(valsToCheck) {
					if slowCheckCh == nil {
						// all values have been received and checked
						return
					}
					// all values have been checked but there are still more to receive
					// Set a long enough wait duration to avoid triggering this until
					// a new new element is received
					waitDuration = time.Now().Add(sendPeriod * 2)
					continue
				}
				waitDuration = valsToCheck[valToReview].time.Add(expiry)
			}
		}
	})

	wg.Wait()
}
