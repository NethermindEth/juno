package pruner

import (
	"context"
	"testing"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/feed"
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
	extraOpts ...Option,
) *servicePruner {
	t.Helper()
	l1Feed := feed.New[*core.L1Head]()
	l2Feed := feed.New[*core.Block]()
	pruned := make(chan pruneEvent, 16)

	opts := append([]Option{
		WithListener(&SelectiveListener{
			OnPruneCb: func(oldest, count uint64, _ time.Duration) {
				pruned <- pruneEvent{oldest: oldest, count: count}
			},
		}),
	}, extraOpts...)

	p := New(
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
		<-done
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

// TestPruner_L1Path covers the L1 head dispatch handler. The L1 path is
// gated by two guards (L1 ahead of chainHeight, L1 inside retention
// window) and only triggers a prune once both pass. The happy path is
// service-driven; the no-op guards are driven via direct onNewL1Head
// calls because no-op dispatches don't fire OnPrune, so the listener
// barrier doesn't apply.
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

		var prunes int
		p := New(
			database,
			10,
			nil, nil,
			log.NewNopZapLogger(),
			WithListener(&SelectiveListener{
				OnPruneCb: func(uint64, uint64, time.Duration) { prunes++ },
			}),
		)

		require.NoError(t, p.onNewL1Head(t.Context(), &core.L1Head{BlockNumber: 50}))
		assert.Zero(t, prunes)
	})
	t.Run("L1 head inside the retention window is a no-op", func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		blocks := make([]*testutils.StoredBlock, totalBlocks)
		for i := range totalBlocks {
			blocks[i] = testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks))

		var prunes int
		p := New(
			database,
			1000, // retention way larger than current L1 head
			nil, nil,
			log.NewNopZapLogger(),
			WithListener(&SelectiveListener{
				OnPruneCb: func(uint64, uint64, time.Duration) { prunes++ },
			}),
		)

		require.NoError(t, p.onNewL1Head(t.Context(), &core.L1Head{BlockNumber: 50}))
		assert.Zero(t, prunes)
		for i := range totalBlocks {
			testutils.AssertBlockExists(t, database, blocks[i])
		}
	})
}

// TestPruner_L2Path covers the L2 head dispatch handler. The L2 path is
// gated by three guards (L2 ahead of L1, block too shallow for retention,
// coalesce threshold not reached) and only triggers a prune once all
// three pass. The happy path is service-driven; the no-op guards are
// driven via direct onNewBlock calls because no-op dispatches don't fire
// OnPrune, so the listener barrier doesn't apply.
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
		sp := startPrunerService(t, database, 10, WithL2HeadsPerPrune(1))
		ev := sp.sendL2AndAwait(t, 90)
		assert.Equal(t, uint64(81), ev.oldest)
		assert.Equal(t, uint64(81), ev.count)
	})

	t.Run("coalesces N L2 heads before triggering one prune", func(t *testing.T) {
		// Direct onNewBlock calls — silent no-op dispatches have no listener
		// barrier, so we drive the dispatch synchronously.
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteL1Head(database, &core.L1Head{BlockNumber: 95}))

		var prunes int
		p := New(
			database,
			10,
			nil, nil,
			log.NewNopZapLogger(),
			WithL2HeadsPerPrune(3),
			WithListener(&SelectiveListener{
				OnPruneCb: func(uint64, uint64, time.Duration) { prunes++ },
			}),
		)

		// First two L2 heads coalesce silently; the third one tips the
		// counter and triggers a prune.
		for i := uint64(88); i <= 90; i++ {
			require.NoError(t, p.onNewBlock(t.Context(), &core.Block{
				Header: &core.Header{Number: i},
			}))
		}
		assert.Equal(t, 1, prunes, "exactly one prune dispatch after threshold reached")

		// The next two events coalesce again (counter reset after the prune);
		// the third triggers a second prune.
		for i := uint64(91); i <= 93; i++ {
			require.NoError(t, p.onNewBlock(t.Context(), &core.Block{
				Header: &core.Header{Number: i},
			}))
		}
		assert.Equal(t, 2, prunes, "counter resets after each prune")
	})

	noopCases := []struct {
		name      string
		l1Head    uint64
		retention uint64
		l2Block   uint64
		why       string
	}{
		{
			name: "L2 ahead of L1", l1Head: 50, retention: 10, l2Block: 60,
			why: "L2 ahead of L1 — block not yet L1-confirmed, floor can't move",
		},
		{
			name: "block shallower than retention", l1Head: 95, retention: 50, l2Block: 20,
			why: "block.Number < retention would underflow oldestToKeep",
		},
	}
	for _, tc := range noopCases {
		t.Run(tc.name+" is a no-op", func(t *testing.T) {
			database := testutils.NewPebbleTestDB(t)
			for i := range totalBlocks {
				testutils.StoreBlock(t, database, i)
			}
			require.NoError(t, core.WriteL1Head(database, &core.L1Head{BlockNumber: tc.l1Head}))

			var prunes int
			p := New(
				database,
				tc.retention,
				nil, nil,
				log.NewNopZapLogger(),
				WithL2HeadsPerPrune(1),
				WithListener(&SelectiveListener{
					OnPruneCb: func(uint64, uint64, time.Duration) { prunes++ },
				}),
			)

			require.NoError(t, p.onNewBlock(t.Context(), &core.Block{
				Header: &core.Header{Number: tc.l2Block},
			}))
			assert.Zero(t, prunes, tc.why)
		})
	}
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
