package tracker_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils/tracker"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/require"
)

const (
	runtimeDelay              = 100 * time.Millisecond
	acquireDelay              = runtimeDelay * 2
	requests                  = 100
	nonDelayedRequests uint32 = 50
)

type waitGroup interface {
	Add() bool
	Done()
	Wait()
}

func TestTracker(t *testing.T) {
	runTest(t, func(context.Context) waitGroup {
		return &tracker.Tracker{}
	})
}

func TestSemaphoreWrapper(t *testing.T) {
	runTest(t, func(ctx context.Context) waitGroup {
		return NewSemaphoreWrapper(ctx)
	})
}

func TestWaitGroupFailure(t *testing.T) {
	t.Skip("This test is skipped because it's supposed to fail to demonstrate the issue")
	runTest(t, func(context.Context) waitGroup {
		return &waitGroupWrapper{}
	})
}

func runTest(t *testing.T, tracker func(context.Context) waitGroup) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	isShutdown := atomic.Bool{}
	success := atomic.Uint32{}
	service := NewService(tracker(ctx), &isShutdown)

	wg := conc.NewWaitGroup()

	wg.Go(func() {
		service.run(ctx)
		// Set isShutdown to true after finishing wait, so we can panic any subsequent acquisitions
		isShutdown.Store(true)
	})

	wg.Go(func() {
		// Wait until all ctx.Done() checks have passed before canceling the context
		for range requests {
			<-service.passedCtxCheck
		}
		cancel()
	})

	for range requests {
		wg.Go(func() {
			if result := service.handle(ctx); result {
				success.Add(1)
			}
		})
	}

	wg.Wait()

	// This is to make sure that at least nonDelayedRequests requests have been acquired
	require.GreaterOrEqual(t, success.Load(), nonDelayedRequests)
}
