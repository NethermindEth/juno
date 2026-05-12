package pruner_test

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
)

// startStalenessPruner spins up a Pruner with an empty pebble DB and feeds,
// returning a counter for OnL1Stale invocations and the L1 feed (for tests
// that need to reset the staleness ticker mid-flight). Run is launched in a
// goroutine inside the synctest bubble; t.Cleanup cancels it.
//
// The DB is left empty deliberately: with no chain height stored,
// onNewL1Head short-circuits on db.ErrKeyNotFound, so L1 events become
// pure ticker-reset triggers and never enter pruneUpto.
func startStalenessPruner(t *testing.T) (*atomic.Int64, *feed.Feed[*core.L1Head]) {
	t.Helper()

	database := testutils.NewPebbleTestDB(t)
	l1Feed := feed.New[*core.L1Head]()
	l2Feed := feed.New[*core.Block]()

	staleCount := &atomic.Int64{}
	p := pruner.New(
		database,
		64,
		l2Feed.Subscribe(),
		l1Feed.Subscribe(),
		log.NewNopZapLogger(),
		pruner.WithListener(&pruner.SelectiveListener{
			OnL1StaleCb: func() { staleCount.Add(1) },
			OnPruneErrorCb: func(err error) {
				t.Errorf("unexpected pruner handler error: %v", err)
			},
		}),
	)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("pruner Run returned error: %v", err)
		}
	})

	// Let Run reach its select{} on the staleness ticker before the first
	// time advance, otherwise the bubble could try to advance the clock
	// before the ticker exists.
	synctest.Wait()
	return staleCount, l1Feed
}

func TestStalenessFiresAfter24Hours(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		staleCount, _ := startStalenessPruner(t)

		time.Sleep(24*time.Hour - time.Second)
		synctest.Wait()
		require.Equal(t, int64(0), staleCount.Load(), "stale fired before 24h elapsed")

		time.Sleep(2 * time.Second)
		synctest.Wait()
		require.Equal(t, int64(1), staleCount.Load(), "stale did not fire after 24h")
	})
}

func TestStalenessSilentWhenL1HeadArrivesInTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		staleCount, l1Feed := startStalenessPruner(t)

		time.Sleep(23 * time.Hour)
		synctest.Wait()

		l1Feed.Send(&core.L1Head{BlockNumber: 1})
		synctest.Wait()

		// 23h more — 46h since start, but only 23h since the reset.
		time.Sleep(23 * time.Hour)
		synctest.Wait()
		require.Equal(t, int64(0), staleCount.Load(), "stale fired despite L1 head reset")

		// Past 24h since reset — should fire exactly once.
		time.Sleep(time.Hour + time.Second)
		synctest.Wait()
		require.Equal(t, int64(1), staleCount.Load(), "stale did not fire 24h after reset")
	})
}

func TestStalenessFiresHourlyAfterFirstFire(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		staleCount, _ := startStalenessPruner(t)

		time.Sleep(24*time.Hour + time.Second)
		synctest.Wait()
		require.Equal(t, int64(1), staleCount.Load(), "first stale did not fire at 24h")

		time.Sleep(time.Hour)
		synctest.Wait()
		require.Equal(t, int64(2), staleCount.Load(), "second stale did not fire 1h after first")

		time.Sleep(time.Hour)
		synctest.Wait()
		require.Equal(t, int64(3), staleCount.Load(), "third stale did not fire 1h after second")
	})
}
