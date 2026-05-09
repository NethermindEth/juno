package pruner_test

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pruneEvent is the payload reported via OnPrune; tests use the sync
// channel as a barrier to wait for each prune dispatch in service mode.
type pruneEvent struct {
	oldest uint64
	count  uint64
}

// servicePruner runs a Pruner via Pruner.Run as a real service and exposes
// the two trigger feeds plus a barrier channel that fires on every prune
// dispatch (including no-op prunes where blocksPruned == 0). t.Cleanup
// cancels Run and waits for it to exit.
type servicePruner struct {
	l1Feed *feed.Feed[*core.L1Head]
	l2Feed *feed.Feed[*core.Block]
	pruned chan pruneEvent
}

func startPrunerService(
	t *testing.T,
	database db.KeyValueStore,
	retention uint64,
	extraOpts ...pruner.Option,
) *servicePruner {
	t.Helper()
	l1Feed := feed.New[*core.L1Head]()
	l2Feed := feed.New[*core.Block]()
	pruned := make(chan pruneEvent, 16)

	opts := append([]pruner.Option{
		pruner.WithListener(&pruner.SelectiveListener{
			OnPruneCb: func(oldest, count uint64, _ time.Duration) {
				pruned <- pruneEvent{oldest: oldest, count: count}
			},
			// Handler errors are logged-and-continued by Run, so without this
			// hook a broken test setup would surface only as an OnPrune
			// timeout. Fail loud instead.
			OnPruneErrorCb: func(err error) {
				t.Errorf("pruner handler error: %v", err)
			},
		}),
	}, extraOpts...)

	p := pruner.New(
		database,
		retention,
		l2Feed.Subscribe(),
		l1Feed.Subscribe(),
		log.NewNopZapLogger(),
		opts...,
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

	return &servicePruner{l1Feed: l1Feed, l2Feed: l2Feed, pruned: pruned}
}

// sendL1AndAwait broadcasts an L1 head to the pruner and waits for the
// resulting prune dispatch. Blocks until OnPrune fires (5s timeout).
func (sp *servicePruner) sendL1AndAwait(t *testing.T, blockNum uint64) pruneEvent {
	t.Helper()
	sp.l1Feed.Send(&core.L1Head{BlockNumber: blockNum})
	select {
	case ev := <-sp.pruned:
		return ev
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for prune dispatch after L1 head %d", blockNum)
		return pruneEvent{}
	}
}

// sendL2AndAwait broadcasts an L2 head to the pruner and waits for the
// resulting prune dispatch. Use only when the configured
// WithL2HeadsPerPrune is 1 (every L2 head triggers OnPrune); higher
// thresholds coalesce events silently and provide no listener barrier.
func (sp *servicePruner) sendL2AndAwait(t *testing.T, blockNum uint64) pruneEvent {
	t.Helper()
	sp.l2Feed.Send(&core.Block{Header: &core.Header{Number: blockNum}})
	select {
	case ev := <-sp.pruned:
		return ev
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for prune dispatch after L2 head %d", blockNum)
		return pruneEvent{}
	}
}

// noOpQuietWindow is how long the sendAndExpectNoOp helpers wait before declaring "no prune fired."
const noOpQuietWindow = 200 * time.Millisecond

// sendL1AndExpectNoOp broadcasts an L1 head and verifies no prune dispatch
// fires within noOpQuietWindow. Use for events that hit a guard
// short-circuit before reaching pruneUpto, since no-op dispatches don't
// fire OnPrune and so provide no listener barrier.
func (sp *servicePruner) sendL1AndExpectNoOp(t *testing.T, blockNum uint64) {
	t.Helper()
	sp.l1Feed.Send(&core.L1Head{BlockNumber: blockNum})
	select {
	case ev := <-sp.pruned:
		t.Fatalf("unexpected prune after L1 head %d: %+v", blockNum, ev)
	case <-time.After(noOpQuietWindow):
	}
}

// sendL2AndExpectNoOp broadcasts an L2 head and verifies no prune dispatch
// fires within noOpQuietWindow. Use for events that hit a guard
// short-circuit before reaching pruneUpto.
func (sp *servicePruner) sendL2AndExpectNoOp(t *testing.T, blockNum uint64) {
	t.Helper()
	sp.l2Feed.Send(&core.Block{Header: &core.Header{Number: blockNum}})
	select {
	case ev := <-sp.pruned:
		t.Fatalf("unexpected prune after L2 head %d: %+v", blockNum, ev)
	case <-time.After(noOpQuietWindow):
	}
}

// TestPruner_L1Path covers the L1 head dispatch handler. The L1 path is
// gated by three guards (chain height missing, L1 ahead of chainHeight,
// L1 inside retention window) and only triggers a prune once all pass.
// All cases are service-driven; the happy path uses the OnPrune barrier,
// the guard-no-op cases use sendL1AndExpectNoOp's quiet-window check.
func TestPruner_L1Path(t *testing.T) {
	const totalBlocks uint64 = 100

	t.Run("L1 head triggers prune and reports via listener", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		blocks := make([]*testutils.StoredBlock, totalBlocks)
		for i := range totalBlocks {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks))

		// retention=10, L1=95 → floor = 95-10+1 = 86. Prunes [0, 86).
		sp := startPrunerService(t, database, 10)
		ev := sp.sendL1AndAwait(t, 95)
		assert.Equal(t, uint64(86), ev.oldest)
		assert.Equal(t, uint64(86), ev.count)

		for i := range uint64(86) - core.BlockHashLag {
			testutils.AssertBlockPruned(t, database, blocks[i])
		}
	})

	t.Run("L1 head at or beyond chainHeight is a no-op", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, 50))

		sp := startPrunerService(t, database, 10)
		sp.sendL1AndExpectNoOp(t, 50)
	})
	t.Run("L1 head inside the retention window is a no-op", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		blocks := make([]*testutils.StoredBlock, totalBlocks)
		for i := range totalBlocks {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks))

		// retention way larger than current L1 head → guard short-circuits.
		sp := startPrunerService(t, database, 1000)
		sp.sendL1AndExpectNoOp(t, 50)

		for i := range totalBlocks {
			testutils.AssertBlockExists(t, database, blocks[i])
		}
	})

	t.Run("L1 head with no chain height in DB is a no-op", func(t *testing.T) {
		// Deliberately omit core.WriteChainHeight: onNewL1Head must
		// short-circuit on ErrKeyNotFound instead of surfacing it to the
		// listener (OnPruneErrorCb fails the test on any handler error).
		database := testutils.NewPebbleTestDB(t)

		sp := startPrunerService(t, database, 10)
		sp.sendL1AndExpectNoOp(t, 95)
	})
}

// TestPruner_L2Path covers the L2 head dispatch handler. The L2 path is
// gated by four guards (L1 head missing, L2 ahead of L1, block too
// shallow for retention, coalesce threshold not reached) and only
// triggers a prune once all pass. All cases are service-driven; the
// happy path and the coalesce test use the OnPrune barrier on the
// trigger event, the guard-no-op cases use sendL2AndExpectNoOp's
// quiet-window check.
func TestPruner_L2Path(t *testing.T) {
	const totalBlocks uint64 = 100

	t.Run("threshold=1 fires a prune on every L2 head", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks))
		require.NoError(t, core.WriteL1Head(database, &core.L1Head{BlockNumber: 95}))

		// retention=10, l1Head=95 → L2 path uses the L2 block as the floor
		// pivot. Sending L2 head 90 → floor = 90-10+1 = 81. Prunes [0, 81).
		sp := startPrunerService(t, database, 10, pruner.WithL2HeadsPerPrune(1))
		ev := sp.sendL2AndAwait(t, 90)
		assert.Equal(t, uint64(81), ev.oldest)
		assert.Equal(t, uint64(81), ev.count)
	})

	t.Run("coalesces N L2 heads before triggering one prune", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks))
		require.NoError(t, core.WriteL1Head(database, &core.L1Head{BlockNumber: 95}))

		// retention=10, l1Head=95, threshold=3. With L2 heads 88, 89, 90:
		// the first two coalesce silently; the third tips the counter and
		// triggers a prune at floor 90-10+1 = 81.
		sp := startPrunerService(t, database, 10, pruner.WithL2HeadsPerPrune(3))
		sp.sendL2AndExpectNoOp(t, 88)
		sp.sendL2AndExpectNoOp(t, 89)
		ev1 := sp.sendL2AndAwait(t, 90)
		assert.Equal(t, uint64(81), ev1.oldest)
		assert.Equal(t, uint64(81), ev1.count)

		// Counter resets after the prune; next two coalesce, the third
		// triggers a second prune at floor 93-10+1 = 84, deleting [81, 84).
		sp.sendL2AndExpectNoOp(t, 91)
		sp.sendL2AndExpectNoOp(t, 92)
		ev2 := sp.sendL2AndAwait(t, 93)
		assert.Equal(t, uint64(84), ev2.oldest)
		assert.Equal(t, uint64(3), ev2.count, "counter resets after each prune")
	})

	noopCases := []struct {
		name      string
		l1Head    uint64
		retention uint64
		l2Block   uint64
	}{
		{name: "L2 ahead of L1", l1Head: 50, retention: 10, l2Block: 60},
		{name: "block shallower than retention", l1Head: 95, retention: 50, l2Block: 20},
	}
	for _, tc := range noopCases {
		t.Run(tc.name+" is a no-op", func(t *testing.T) {
			database := testutils.NewPebbleTestDB(t)
			for i := range totalBlocks {
				testutils.StoreBlock(t, database, i)
			}
			require.NoError(t, core.WriteChainHeight(database, totalBlocks))
			require.NoError(t, core.WriteL1Head(database, &core.L1Head{BlockNumber: tc.l1Head}))

			sp := startPrunerService(t, database, tc.retention, pruner.WithL2HeadsPerPrune(1))
			sp.sendL2AndExpectNoOp(t, tc.l2Block)
		})
	}

	t.Run("L2 head with no L1 head in DB is a no-op", func(t *testing.T) {
		// Deliberately omit core.WriteL1Head: onNewBlock must short-circuit
		// on ErrKeyNotFound instead of surfacing it to the listener
		// (OnPruneErrorCb fails the test on any handler error).
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks))

		sp := startPrunerService(t, database, 10, pruner.WithL2HeadsPerPrune(1))
		sp.sendL2AndExpectNoOp(t, 90)
	})
}

// TestPruner_RetentionChangeAcrossRestart simulates a node restart with a
// different --retained-blocks value. Drives the pruner via the real
// service: a goroutine running Pruner.Run consumes L1 head events from
// the feed, and each phase is synchronized via the OnPrune listener.
//
//  1. Shrinking the window (80 → 10) triggers a backlog sweep on the
//     next L1 event: blocks between the old and new floor are pruned in
//     a single dispatch.
//  2. Growing the window (10 → 40) is a no-op until L1 advances past
//     the new (lower) floor — the window grows gradually, one L1 step
//     at a time. The pruneUpto wrapper still calls OnPrune (with
//     blocksPruned == 0) on the no-op paths, so the barrier works.
//
// numRetainedBlocks is held only in memory, never persisted, so a restart
// with a different value is just a new Pruner on the same DB.
func TestPruner_RetentionChangeAcrossRestart(t *testing.T) {
	const totalBlocks uint64 = 100
	lag := core.BlockHashLag

	t.Run("shrinking retention drains backlog on next L1 event", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		blocks := make([]*testutils.StoredBlock, totalBlocks)
		for i := range totalBlocks {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks))

		// Phase 1: retention=80, L1=99 → floor=20. Sweep prunes [0, 20).
		sp1 := startPrunerService(t, database, 80)
		ev1 := sp1.sendL1AndAwait(t, 99)
		assert.Equal(t, uint64(20), ev1.oldest)
		assert.Equal(t, uint64(20), ev1.count)

		// Phase 2: "restart" with retention=10, L1=99 → floor=90. Sweep
		// prunes [20, 90) — 70 blocks in one dispatch.
		sp2 := startPrunerService(t, database, 10)
		ev2 := sp2.sendL1AndAwait(t, 99)
		assert.Equal(t, uint64(90), ev2.oldest)
		assert.Equal(t, uint64(70), ev2.count)

		// End-state: everything below the new lag floor is fully gone,
		// blocks ≥ 90 untouched.
		for i := range uint64(90) - lag {
			testutils.AssertBlockPruned(t, database, blocks[i])
		}
		for i := uint64(90); i < totalBlocks; i++ {
			testutils.AssertBlockExists(t, database, blocks[i])
		}
	})

	t.Run("growing retention pauses pruning until L1 catches up", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks))

		// Phase 1: retention=10, L1=50 → floor=41. Sweep prunes [0, 41).
		sp1 := startPrunerService(t, database, 10)
		ev1 := sp1.sendL1AndAwait(t, 50)
		assert.Equal(t, uint64(41), ev1.oldest)
		assert.Equal(t, uint64(41), ev1.count)

		// Phase 2: "restart" with retention=40, L1 still at 50 → new floor=11,
		// below the existing oldestKept (41). PruneUpto early-returns; OnPrune
		// fires with count=0 and oldest unchanged at 41.
		sp2 := startPrunerService(t, database, 40)
		ev2 := sp2.sendL1AndAwait(t, 50)
		assert.Equal(t, uint64(41), ev2.oldest,
			"growing retention must not resurrect or move oldestKept")
		assert.Zero(t, ev2.count, "growing retention must not prune any block")

		// Phase 3: L1 advances to 81 → floor=42 > 41. The window finally moves
		// by exactly one block: gradual growth, not a leap.
		ev3 := sp2.sendL1AndAwait(t, 81)
		assert.Equal(t, uint64(42), ev3.oldest)
		assert.Equal(t, uint64(1), ev3.count)
	})
}
