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
	nonDelayed         = 10
	delayed            = 100
	asyncBoundaryDelay = 1 * time.Millisecond
)

type waitGroup interface {
	Add(delta int) bool
	Done()
	Wait()
}

func TestTracker(t *testing.T) {
	runTests(t, func(context.Context) waitGroup {
		return &tracker.Tracker{}
	})
}

func TestSemaphoreWrapper(t *testing.T) {
	runTests(t, func(ctx context.Context) waitGroup {
		return NewSemaphoreWrapper(ctx)
	})
}

func TestWaitGroupFailure(t *testing.T) {
	t.Skip("This test is skipped because it's supposed to fail to demonstrate the issue")
	runTests(t, func(context.Context) waitGroup {
		return &waitGroupWrapper{}
	})
}

func runTests(t *testing.T, tracker func(context.Context) waitGroup) {
	t.Helper()

	t.Run("normal case, all requests complete", func(t *testing.T) {
		runTest(t, false, nonDelayed, 0, nonDelayed, tracker)
	})

	t.Run("early cancel, half of non-delayed requests complete", func(t *testing.T) {
		runTest(t, true, nonDelayed, delayed, nonDelayed/2, tracker)
	})
}

func runTest(
	t *testing.T,
	earlyCancel bool,
	nonDelayed int,
	delayed int,
	expected int,
	tracker func(context.Context) waitGroup,
) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	isShutdown := atomic.Bool{}
	success := atomic.Uint32{}
	service := NewService(tracker(ctx), &isShutdown, nonDelayed, delayed)

	clientWg := conc.NewWaitGroup()
	serverWg := conc.NewWaitGroup()

	serverWg.Go(func() {
		service.run(ctx)
		// Set isShutdown to true after finishing wait, so we can panic any subsequent acquisitions
		isShutdown.Store(true)
	})

	if earlyCancel {
		serverWg.Go(func() {
			// Wait until all ctx.Done() checks have passed before canceling the context
			service.passedCtxCheck.done()
			cancel()
		})
	}

	for isDelayed, count := range map[bool]int{false: nonDelayed, true: delayed} {
		for range count {
			clientWg.Go(func() {
				if result := service.handle(ctx, isDelayed); result {
					success.Add(1)
				}
			})
		}
	}

	clientWg.Wait()
	cancel()
	serverWg.Wait()

	require.GreaterOrEqual(t, int(success.Load()), expected)
}
