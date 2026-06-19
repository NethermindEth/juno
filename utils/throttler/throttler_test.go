package throttler_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils/throttler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThrottler(t *testing.T) {
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
	do := func() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			require.NoError(t, throttledRes.Do(ctx, doer))
		}()
		time.Sleep(time.Millisecond)
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
	time.Sleep(time.Millisecond)
	assert.Equal(t, 1, throttledRes.QueueLen())
	waitOn <- struct{}{} // release another slot, qeueue should be empty
	time.Sleep(time.Millisecond)
	assert.Equal(t, 0, throttledRes.QueueLen())

	// release the jobs waiting
	waitOn <- struct{}{}
	waitOn <- struct{}{}
	wg.Wait()
	assert.Equal(t, int64(4), runCount)
}

func TestThrottlerContextCancellation(t *testing.T) {
	// Budget of 1 and a generous queue, so the second call is queued (waiting for
	// the slot) rather than rejected with ErrResourceBusy.
	throttledRes := throttler.NewThrottler(1, new(int), throttler.WithMaxQueueLen(10))
	waitOn := make(chan struct{})

	blockingDoer := func(*int) error {
		<-waitOn
		return nil
	}

	// Occupy the only slot with a job that blocks until released.
	var wg sync.WaitGroup
	wg.Go(func() {
		require.NoError(t, throttledRes.Do(t.Context(), blockingDoer))
	})
	time.Sleep(time.Millisecond)
	assert.Equal(t, 1, throttledRes.JobsRunning())

	// The second call blocks waiting for the slot; cancel its context while queued.
	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() {
		errCh <- throttledRes.Do(ctx, blockingDoer)
	}()
	time.Sleep(time.Millisecond)
	assert.Equal(t, 1, throttledRes.QueueLen())

	cancel()
	require.ErrorIs(t, <-errCh, context.Canceled)
	time.Sleep(time.Millisecond)
	assert.Equal(t, 0, throttledRes.QueueLen()) // queue is decremented on cancellation

	// Release the running job.
	waitOn <- struct{}{}
	wg.Wait()
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
