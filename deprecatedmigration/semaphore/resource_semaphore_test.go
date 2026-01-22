package semaphore_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/deprecatedmigration/semaphore"
	"github.com/stretchr/testify/require"
)

const (
	concurrency = 4
	loop        = 10000
)

func TestResourceSemaphore(t *testing.T) {
	t.Run("Normal usage", func(t *testing.T) {
		counter := atomic.Int32{}

		sem := semaphore.New(
			concurrency,
			func() int {
				current := counter.Add(1)
				require.LessOrEqual(t, current, int32(concurrency))
				return 1
			},
		)

		wg := sync.WaitGroup{}
		for range concurrency * 4 {
			wg.Go(func() {
				for range loop {
					res := sem.GetBlocking()
					require.Equal(t, 1, res)
					counter.Add(-1)
					sem.Put()
				}
			})
		}
		wg.Wait()
	})

	t.Run("Context cancellation", func(t *testing.T) {
		sem := semaphore.New(
			concurrency,
			func() int { return 1 },
		)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := sem.Get(ctx)
		require.ErrorIs(t, err, context.Canceled)
	})
}
