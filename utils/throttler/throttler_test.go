package throttler_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/NethermindEth/juno/utils/throttler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThrottler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		throttledRes := throttler.NewThrottler(2, new(int), throttler.WithMaxQueueLen(2))
		ctx := t.Context()
		waitOn := make(chan struct{})

		var runCount int64
		doer := func(ptr *int) error {
			if ptr == nil {
				return errors.New("nilptr")
			}
			<-waitOn
			atomic.AddInt64(&runCount, 1)
			return nil
		}

		var wg sync.WaitGroup
		// do spawns a Do call, then waits for the bubble to settle so the
		// throttler's queue length is observed deterministically.
		do := func() {
			wg.Go(func() {
				// assert, not require: runs off the test goroutine
				assert.NoError(t, throttledRes.Do(ctx, doer))
			})
			synctest.Wait() // block until the spawned goroutine is durably blocked
		}

		do()
		assert.Equal(t, 0, throttledRes.QueueLen())
		do()
		assert.Equal(t, 0, throttledRes.QueueLen())

		do() // should be queued
		assert.Equal(t, 1, throttledRes.QueueLen())
		do() // should be queued
		assert.Equal(t, 2, throttledRes.QueueLen())

		require.ErrorIs(t, throttledRes.Do(ctx, doer), throttler.ErrResourceBusy)

		waitOn <- struct{}{} // release one of the slots
		synctest.Wait()
		assert.Equal(t, 1, throttledRes.QueueLen())
		waitOn <- struct{}{} // release another slot, queue should be empty
		synctest.Wait()
		assert.Equal(t, 0, throttledRes.QueueLen())

		// release the jobs waiting
		waitOn <- struct{}{}
		waitOn <- struct{}{}
		wg.Wait()
		assert.Equal(t, int64(4), runCount)
	})
}

func TestThrottlerContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Budget of 1 and a generous queue, so the second call is queued (waiting
		// for the slot) rather than rejected with ErrResourceBusy.
		throttledRes := throttler.NewThrottler(1, new(int), throttler.WithMaxQueueLen(10))
		waitOn := make(chan struct{})

		//nolint: unparam // defined this way to satisfy a function signature
		blockingDoer := func(*int) error {
			<-waitOn
			return nil
		}

		// Occupy the only slot with a job that blocks until released.
		var wg sync.WaitGroup
		wg.Go(func() {
			assert.NoError(t, throttledRes.Do(t.Context(), blockingDoer))
		})
		synctest.Wait()
		assert.Equal(t, 1, throttledRes.JobsRunning())

		// The second call blocks waiting for the slot; cancel its context while queued.
		ctx, cancel := context.WithCancel(t.Context())
		errCh := make(chan error, 1)
		go func() {
			errCh <- throttledRes.Do(ctx, blockingDoer)
		}()
		synctest.Wait()
		assert.Equal(t, 1, throttledRes.QueueLen())

		cancel()
		require.ErrorIs(t, <-errCh, context.Canceled)
		synctest.Wait()
		assert.Equal(t, 0, throttledRes.QueueLen()) // queue is decremented on cancellation

		// Release the running job.
		waitOn <- struct{}{}
		wg.Wait()
	})
}

func TestThrottlerAlreadyCancelledContext(t *testing.T) {
	throttledRes := throttler.NewThrottler(1, new(int))
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancelled before Do is called

	ran := false
	err := throttledRes.Do(ctx, func(*int) error {
		ran = true
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, ran) // the doer is never invoked
	assert.Equal(t, 0, throttledRes.JobsRunning())
	assert.Equal(t, 0, throttledRes.QueueLen())
}

func TestThrottlerZeroQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Budget of 2 with no queue: up to 2 calls run concurrently, anything
		// beyond is rejected immediately rather than waiting.
		throttledRes := throttler.NewThrottler(2, new(int), throttler.WithMaxQueueLen(0))
		waitOn := make(chan struct{})

		//nolint: unparam // defined this way to satisfy a function signature
		blockingDoer := func(*int) error {
			<-waitOn
			return nil
		}

		var wg sync.WaitGroup
		run := func() {
			wg.Go(func() {
				assert.NoError(t, throttledRes.Do(t.Context(), blockingDoer))
			})
			synctest.Wait()
		}

		// Both calls fit within the concurrency budget and run despite the 0 queue.
		run()
		assert.Equal(t, 1, throttledRes.JobsRunning())
		run()
		assert.Equal(t, 2, throttledRes.JobsRunning())
		assert.Equal(t, 0, throttledRes.QueueLen())

		// Budget is full and the queue allows nothing, so the next call is rejected.
		require.ErrorIs(t, throttledRes.Do(t.Context(), blockingDoer), throttler.ErrResourceBusy)

		// Release the running jobs.
		waitOn <- struct{}{}
		waitOn <- struct{}{}
		wg.Wait()
	})
}

func TestThrottlerMaxQueueLenOverflow(t *testing.T) {
	// maxQueueLen + budget must not overflow: MaxUint64 + 1 wraps to 0, which
	// without the guard would reject every call. The guard treats an overflowing
	// total as unbounded, so a call within the budget still runs.
	throttledRes := throttler.NewThrottler(1, new(int), throttler.WithMaxQueueLen(math.MaxUint64))
	require.NoError(t, throttledRes.Do(t.Context(), func(*int) error { return nil }))
	assert.Equal(t, 0, throttledRes.QueueLen())
}
