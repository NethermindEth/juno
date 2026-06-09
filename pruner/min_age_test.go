package pruner_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/pruner/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMinAge is the minimum-age window used by these tests. Inside a
// synctest bubble both the pruner's ticker and time.Now() advance only
// via virtual time.Sleep, so the value doesn't need to be small.
const testMinAge = 1 * time.Hour

func TestPruner_MinAgeFloor(t *testing.T) {
	t.Run("tick refreshes floor", testMinAgeFloorTickRefresh)
	t.Run("init seeds floor to oldest within-window block (restart safety)", testMinAgeFloorInitSeed)
	t.Run("init: ancient chain seeds to head (deep catch-up)", testMinAgeFloorInitAllAncient)
	t.Run("L1 path: floor binds when cached sample is below standard floor", testMinAgeFloorL1Binds)
	t.Run("L1 steady state: floor does not bind when above standard", testMinAgeFloorL1NoBind)
	t.Run("L2 path: floor binds for near-tip block (fresh timestamp)", testMinAgeFloorL2Binds)
	t.Run("L2 path: deep catch-up (ancient timestamp) suppresses floor", testMinAgeFloorL2DeepCatchUp)
	t.Run("prune bumps latestSampledHeight so next tick is safe", testMinAgeFloorPruneBumpsSample)
}

func testMinAgeFloorTickRefresh(t *testing.T) {
	const totalBlocks uint64 = 30
	const retention uint64 = 10
	// Simulates a node that starts with only ancient blocks (deep
	// catch-up), then appends fresh-timestamp blocks at the head as
	// L2 reaches tip. After the next tick, sampleHeight re-runs
	// and the floor moves from the not-found fallback
	// (chainHeight) to the lowest fresh block.
	synctest.Test(t, func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		// Start with only 10 ancient blocks. Init seed will fall back
		// to head (= 9), no time-floor protection.
		const initialBlocks uint64 = 10
		for i := range initialBlocks {
			testutils.StoreBlockWithTimestamp(t, database, i, 1)
		}
		require.NoError(t, core.WriteChainHeight(database, 9))

		const tickInterval = 50 * time.Millisecond
		sp := startPrunerService(t, database, retention,
			pruner.WithMinAge(testMinAge),
			pruner.WithFloorTickInterval(tickInterval),
		)
		synctest.Wait()

		// Append fresh-timestamp blocks 10..29 (as if L2 had caught
		// up to tip and is producing real-time blocks now).
		now := uint64(time.Now().Unix())
		for i := uint64(10); i < totalBlocks; i++ {
			testutils.StoreBlockWithTimestamp(t, database, i, now)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks-1))

		// Advance past one tick — sampleHeight reruns the binary
		// search over [previousFloor=9, newHead=29] and finds block
		// 10 as the lowest fresh block.
		time.Sleep(tickInterval + time.Millisecond)
		synctest.Wait()

		// L1=25, retention=10 → standardFloor=15. Floor=10 < 15 → bind at 10.
		ev := sp.sendL1AndAwait(t, 25)
		assert.Equal(t, uint64(10), ev.oldest,
			"tick must bind to the refreshed seed (10)")
		assert.Equal(t, uint64(10), ev.count)
	})
}

func testMinAgeFloorInitSeed(t *testing.T) {
	const totalBlocks uint64 = 30
	const retention uint64 = 10
	// Simulates a restart where the DB already holds a window of
	// recent blocks. The binary search should find the boundary
	// where Timestamp first crosses now-minAge and seed both slots
	// to that height, so the floor protects [boundary, head] from
	// t=0 — no zero-protection gap after restart.
	synctest.Test(t, func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		now := uint64(time.Now().Unix())
		fresh := now         // within minAge
		ancient := uint64(1) // far older than any cutoff
		// Blocks 0..9 ancient, 10..29 fresh. Binary search should
		// find block 10 as the lowest still within minAge.
		for i := range totalBlocks {
			ts := ancient
			if i >= 10 {
				ts = fresh
			}
			testutils.StoreBlockWithTimestamp(t, database, i, ts)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks-1))

		sp := startPrunerService(t, database, retention, pruner.WithMinAge(testMinAge))
		synctest.Wait()

		// L1=25, retention=10 → standardFloor=15. Seed floor=10
		// (lowest fresh block). 10 < 15 → bind at 10.
		ev := sp.sendL1AndAwait(t, 25)
		assert.Equal(t, uint64(10), ev.oldest,
			"floor must bind to the binary-searched seed (10), not to current head")
		assert.Equal(t, uint64(10), ev.count)
	})
}

func testMinAgeFloorInitAllAncient(t *testing.T) {
	const totalBlocks uint64 = 30
	const retention uint64 = 10
	// A node that's been syncing historical chain has every block
	// timestamped years in the past. Binary search finds nothing
	// fresh, so it returns current head. The L2-path gate then
	// suppresses the floor on ancient incoming blocks anyway.
	synctest.Test(t, func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlockWithTimestamp(t, database, i, 1) // all ancient
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks-1))

		sp := startPrunerService(t, database, retention, pruner.WithMinAge(testMinAge))
		synctest.Wait()

		// L1=25, retention=10 → standardFloor=15. Seed=head=29
		// (no fresh block). 29 >= 15 → no bind, prune to 15.
		ev := sp.sendL1AndAwait(t, 25)
		assert.Equal(t, uint64(15), ev.oldest)
	})
}

func testMinAgeFloorL1Binds(t *testing.T) {
	const totalBlocks uint64 = 30
	const retention uint64 = 10
	synctest.Test(t, func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		// chainHeight=5 at startup → init sample caches 5.
		require.NoError(t, core.WriteChainHeight(database, 5))

		sp := startPrunerService(t, database, retention, pruner.WithMinAge(testMinAge))
		synctest.Wait()

		// chainHeight bumped to 30; the ticker hasn't fired yet,
		// so cached height stays at 5.
		require.NoError(t, core.WriteChainHeight(database, totalBlocks))
		ev := sp.sendL1AndAwait(t, 25)
		// L1=25, retention=10 → standardFloor=15. Cache=5 < 15 → bind at 5.
		assert.Equal(t, uint64(5), ev.oldest, "floor must clamp to cached height (5)")
		assert.Equal(t, uint64(5), ev.count)
	})
}

func testMinAgeFloorL1NoBind(t *testing.T) {
	const totalBlocks uint64 = 30
	const retention uint64 = 10
	synctest.Test(t, func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		// chainHeight = highest stored block (= totalBlocks-1). All
		// blocks have Timestamp=0 so the init search returns
		// ErrNoBlockInWindow → floor parks at chainHeight.
		require.NoError(t, core.WriteChainHeight(database, totalBlocks-1))

		sp := startPrunerService(t, database, retention, pruner.WithMinAge(testMinAge))
		synctest.Wait()

		// L1=25, retention=10 → standardFloor=15. Cache=29 ≥ 15
		// → no override.
		ev := sp.sendL1AndAwait(t, 25)
		assert.Equal(t, uint64(15), ev.oldest,
			"ev.oldest must equal standardFloor when cached height is above it")
	})
}

func testMinAgeFloorL2Binds(t *testing.T) {
	const totalBlocks uint64 = 30
	const retention uint64 = 10
	synctest.Test(t, func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, 5))
		require.NoError(t, core.WriteL1Head(database, &core.L1Head{BlockNumber: 25}))

		sp := startPrunerService(t, database, retention,
			pruner.WithMinAge(testMinAge),
			pruner.WithL2HeadsPerPrune(1),
		)
		synctest.Wait()

		require.NoError(t, core.WriteChainHeight(database, totalBlocks))
		// L2=20 with fresh timestamp; L1=25 > L2 → L2 path.
		// standardFloor = 20-10 = 10. Cache=5 < 10 → bind at 5.
		ev := sp.sendL2WithTimestampAndAwait(t, 20, uint64(time.Now().Unix()))
		assert.Equal(t, uint64(5), ev.oldest)
		assert.Equal(t, uint64(5), ev.count)
	})
}

// testMinAgeFloorPruneBumpsSample asserts that after a prune advances
// past the cached latestSampledHeight, the post-prune bump in pruneUpto
// keeps the next sampleHeight tick from probing a now-pruned block.
// Without the bump, sampleHeight reads a deleted header and the failure
// surfaces through startPrunerService's OnPruneErrorCb (-> t.Errorf).
func testMinAgeFloorPruneBumpsSample(t *testing.T) {
	const retention uint64 = 10
	const initialBlocks uint64 = 5
	const totalBlocks uint64 = 47
	const tickInterval = 50 * time.Millisecond
	const ancientTimestamp uint64 = 1
	synctest.Test(t, func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		// Deep catch-up: blocks 0..4 ancient + chainHeight=4 → seedFloor
		// parks latestSampledHeight at chainHeight (=4) via ErrNoBlockInWindow.
		for i := range initialBlocks {
			testutils.StoreBlockWithTimestamp(t, database, i, ancientTimestamp)
		}
		require.NoError(t, core.WriteChainHeight(database, initialBlocks-1))

		sp := startPrunerService(t, database, retention,
			pruner.WithMinAge(testMinAge),
			pruner.WithFloorTickInterval(tickInterval),
			pruner.WithL2HeadsPerPrune(1),
		)
		synctest.Wait()

		// Catch-up continues: append more ancient blocks (monotonic) and
		// a high L1 head so the L2 path isn't short-circuited.
		for i := initialBlocks; i < totalBlocks; i++ {
			testutils.StoreBlockWithTimestamp(t, database, i, ancientTimestamp)
		}
		require.NoError(t, core.WriteChainHeight(database, totalBlocks-1))
		require.NoError(t, core.WriteL1Head(database, &core.L1Head{BlockNumber: 900}))

		// L2 head at the new tip (block 46) with its on-chain ancient
		// timestamp → suppression → pruneTo = 46-10 = 36. Carve-out keeps
		// headers in [36-BlockHashLag, 36) = [26, 36); headers in [0, 26)
		// are physically deleted.
		ev := sp.sendL2WithTimestampAndAwait(t, totalBlocks-1, ancientTimestamp)
		assert.Equal(t, uint64(36), ev.oldest)

		// Fire one sampleHeight tick. Without the post-prune bump,
		// FindOldestBlockAtOrAfter starts at lower=4, upper=46 and probes
		// mid=25 first — which lies in the deleted gap [0, 26) and surfaces
		// through OnPruneErrorCb as a test failure. With the bump,
		// lower=36 and every probe hits a preserved header.
		time.Sleep(tickInterval + time.Millisecond)
		synctest.Wait()
	})
}

func testMinAgeFloorL2DeepCatchUp(t *testing.T) {
	const totalBlocks uint64 = 30
	const retention uint64 = 10
	synctest.Test(t, func(t *testing.T) {
		database := testutils.NewPebbleTestDB(t)
		for i := range totalBlocks {
			testutils.StoreBlock(t, database, i)
		}
		require.NoError(t, core.WriteChainHeight(database, 5))
		require.NoError(t, core.WriteL1Head(database, &core.L1Head{BlockNumber: totalBlocks}))

		sp := startPrunerService(t, database, retention,
			pruner.WithMinAge(testMinAge),
			pruner.WithL2HeadsPerPrune(1),
		)
		synctest.Wait()

		require.NoError(t, core.WriteChainHeight(database, totalBlocks))
		const ancientTimestamp uint64 = 1 // before any reasonable cutoff
		// L2=20, retention=10 → standardFloor=10. Ancient timestamp
		// suppresses the gate → standardFloor only.
		ev := sp.sendL2WithTimestampAndAwait(t, 20, ancientTimestamp)
		assert.Equal(t, uint64(10), ev.oldest, "deep catch-up: standard floor only")
		assert.Equal(t, uint64(10), ev.count)
	})
}
