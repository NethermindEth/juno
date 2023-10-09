package node

import (
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPebbleMetrics tests the collection of Pebble database metrics and ensures that
// no panics occur during metric collection. It also verifies that all metric fields are
// properly collected and match the expected values.
func TestPebbleMetrics(t *testing.T) {
	var (
		listener = newPebbleListener()
		wg       sync.WaitGroup
		gathered bool
		metrics  *db.PebbleMetrics
	)
	wg.Add(1)
	selectiveListener := &db.SelectiveListener{
		OnPebbleMetricsCb: func(m *db.PebbleMetrics) {
			if gathered {
				return
			}
			listener.gather(m)
			metrics = m
			gathered = true
			wg.Done()
		},
	}

	testDB := pebble.NewMemTest(t).WithListener(selectiveListener)
	// do some arbitrary load in order to have some data in gathered metrics
	for i := 0; i < 1024*32; i++ {
		txn := testDB.NewTransaction(true)
		key := make([]byte, 32)
		rand.Read(key)
		require.NoError(t, txn.Set(key, key))
		require.NoError(t, txn.Commit())
	}

	testDB.Meter(1 * time.Second)
	wg.Wait()

	t.Run("levels", func(t *testing.T) {
		for i := 0; i < len(metrics.Src.Levels); i++ {
			assert.Equal(t, float64(metrics.Src.Levels[i].NumFiles), testutil.ToFloat64(listener.levels.numFiles[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].Size), testutil.ToFloat64(listener.levels.size[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].Score), testutil.ToFloat64(listener.levels.score[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].BytesIn), testutil.ToFloat64(listener.levels.bytesIn[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].BytesIngested), testutil.ToFloat64(listener.levels.bytesIngested[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].BytesMoved), testutil.ToFloat64(listener.levels.bytesMoved[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].BytesRead), testutil.ToFloat64(listener.levels.bytesRead[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].BytesCompacted), testutil.ToFloat64(listener.levels.bytesCompacted[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].BytesFlushed), testutil.ToFloat64(listener.levels.bytesFlushed[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].TablesCompacted), testutil.ToFloat64(listener.levels.tablesCompacted[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].TablesFlushed), testutil.ToFloat64(listener.levels.tablesFlushed[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].TablesIngested), testutil.ToFloat64(listener.levels.tablesIngested[i]))
			assert.Equal(t, float64(metrics.Src.Levels[i].TablesMoved), testutil.ToFloat64(listener.levels.tablesMoved[i]))
		}
	})

	t.Run("compaction", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.Compact.DefaultCount), testutil.ToFloat64(listener.comp.Defaults))
		assert.Equal(t, float64(metrics.Src.Compact.DeleteOnlyCount), testutil.ToFloat64(listener.comp.DeletesOnly))
		assert.Equal(t, float64(metrics.Src.Compact.ElisionOnlyCount), testutil.ToFloat64(listener.comp.ElisionsOnly))
		assert.Equal(t, float64(metrics.Src.Compact.MoveCount), testutil.ToFloat64(listener.comp.Moves))
		assert.Equal(t, float64(metrics.Src.Compact.ReadCount), testutil.ToFloat64(listener.comp.Reads))
		assert.Equal(t, float64(metrics.Src.Compact.RewriteCount), testutil.ToFloat64(listener.comp.Rewrites))
		assert.Equal(t, float64(metrics.Src.Compact.RewriteCount), testutil.ToFloat64(listener.comp.Rewrites))
		assert.Equal(t, float64(metrics.Src.Compact.MultiLevelCount), testutil.ToFloat64(listener.comp.MultiLevels))
		assert.Equal(t, float64(metrics.Src.Compact.EstimatedDebt), testutil.ToFloat64(listener.comp.EstimatedDebt))
		assert.Equal(t, float64(metrics.Src.Compact.InProgressBytes), testutil.ToFloat64(listener.comp.InProgressBytes))
		assert.Equal(t, float64(metrics.Src.Compact.NumInProgress), testutil.ToFloat64(listener.comp.NumInProgress))
		assert.Equal(t, float64(metrics.Src.Compact.MarkedFiles), testutil.ToFloat64(listener.comp.MarkedFiles))
	})

	t.Run("cache", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.BlockCache.Size), testutil.ToFloat64(listener.cache.Size))
		assert.Equal(t, float64(metrics.Src.BlockCache.Count), testutil.ToFloat64(listener.cache.Count))
		assert.Equal(t, float64(metrics.Src.BlockCache.Hits), testutil.ToFloat64(listener.cache.Hits))
		assert.Equal(t, float64(metrics.Src.BlockCache.Misses), testutil.ToFloat64(listener.cache.Misses))
	})

	t.Run("flush", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.Flush.Count), testutil.ToFloat64(listener.flush.Count))
		assert.Equal(t, float64(metrics.Src.Flush.NumInProgress), testutil.ToFloat64(listener.flush.NumInProgress))
		assert.Equal(t, float64(metrics.Src.Flush.AsIngestCount), testutil.ToFloat64(listener.flush.AsIngestCount))
		assert.Equal(t, float64(metrics.Src.Flush.AsIngestTableCount), testutil.ToFloat64(listener.flush.AsIngestTableCount))
		assert.Equal(t, float64(metrics.Src.Flush.AsIngestBytes), testutil.ToFloat64(listener.flush.AsIngestBytes))
		assert.Equal(t, float64(metrics.Src.Flush.WriteThroughput.Bytes), testutil.ToFloat64(listener.flush.BytesProcessed))
		assert.Equal(t, float64(metrics.Src.Flush.WriteThroughput.IdleDuration), testutil.ToFloat64(listener.flush.IdleDuration))
		assert.Equal(t, float64(metrics.Src.Flush.WriteThroughput.WorkDuration), testutil.ToFloat64(listener.flush.WorkDuration))
	})

	t.Run("filter", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.Filter.Hits), testutil.ToFloat64(listener.filter.Hits))
		assert.Equal(t, float64(metrics.Src.Filter.Misses), testutil.ToFloat64(listener.filter.Misses))
	})

	t.Run("memtable", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.MemTable.Size), testutil.ToFloat64(listener.memtable.Size))
		assert.Equal(t, float64(metrics.Src.MemTable.Count), testutil.ToFloat64(listener.memtable.Count))
		assert.Equal(t, float64(metrics.Src.MemTable.ZombieSize), testutil.ToFloat64(listener.memtable.ZombieSize))
		assert.Equal(t, float64(metrics.Src.MemTable.ZombieCount), testutil.ToFloat64(listener.memtable.Zombies))
	})

	t.Run("keys", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.Keys.RangeKeySetsCount), testutil.ToFloat64(listener.keys.RangeKeySets))
		assert.Equal(t, float64(metrics.Src.Keys.TombstoneCount), testutil.ToFloat64(listener.keys.Tombstones))
	})

	t.Run("snapshots", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.Snapshots.Count), testutil.ToFloat64(listener.snapshots.Count))
		assert.Equal(t, float64(metrics.Src.Snapshots.EarliestSeqNum), testutil.ToFloat64(listener.snapshots.EarliestSeqNum))
		assert.Equal(t, float64(metrics.Src.Snapshots.PinnedKeys), testutil.ToFloat64(listener.snapshots.PinnedKeys))
		assert.Equal(t, float64(metrics.Src.Snapshots.PinnedSize), testutil.ToFloat64(listener.snapshots.PinnedSize))
	})

	t.Run("table", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.Table.ObsoleteCount), testutil.ToFloat64(listener.table.ObsoleteCount))
		assert.Equal(t, float64(metrics.Src.Table.ObsoleteSize), testutil.ToFloat64(listener.table.ObsoleteSize))
		assert.Equal(t, float64(metrics.Src.Table.ZombieCount), testutil.ToFloat64(listener.table.ZombieCount))
		assert.Equal(t, float64(metrics.Src.Table.ZombieSize), testutil.ToFloat64(listener.table.ZombieSize))
		assert.Equal(t, float64(metrics.Src.TableCache.Count), testutil.ToFloat64(listener.table.Count))
		assert.Equal(t, float64(metrics.Src.TableCache.Hits), testutil.ToFloat64(listener.table.Hits))
		assert.Equal(t, float64(metrics.Src.TableCache.Misses), testutil.ToFloat64(listener.table.Misses))
		assert.Equal(t, float64(metrics.Src.TableCache.Size), testutil.ToFloat64(listener.table.Size))
		assert.Equal(t, float64(metrics.Src.TableIters), testutil.ToFloat64(listener.table.Iters))
	})

	t.Run("wal", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.WAL.Files), testutil.ToFloat64(listener.wal.Files))
		assert.Equal(t, float64(metrics.Src.WAL.ObsoleteFiles), testutil.ToFloat64(listener.wal.ObsoleteFiles))
		assert.Equal(t, float64(metrics.Src.WAL.ObsoletePhysicalSize), testutil.ToFloat64(listener.wal.ObsoletePhysicalSize))
		assert.Equal(t, float64(metrics.Src.WAL.Size), testutil.ToFloat64(listener.wal.Size))
		assert.Equal(t, float64(metrics.Src.WAL.PhysicalSize), testutil.ToFloat64(listener.wal.PhysicalSize))
		assert.Equal(t, float64(metrics.Src.WAL.BytesIn), testutil.ToFloat64(listener.wal.BytesIn))
		assert.Equal(t, float64(metrics.Src.WAL.BytesWritten), testutil.ToFloat64(listener.wal.BytesWritten))
	})

	t.Run("disk", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.DiskSpaceUsage()), testutil.ToFloat64(listener.disk.diskUsage))
	})

	t.Run("logs", func(t *testing.T) {
		assert.Equal(t, float64(metrics.Src.LogWriter.WriteThroughput.Bytes), testutil.ToFloat64(listener.logs.Bytes))
		assert.Equal(t, float64(metrics.Src.LogWriter.WriteThroughput.WorkDuration.Seconds()), testutil.ToFloat64(listener.logs.WorkDuration))
		assert.Equal(t, float64(metrics.Src.LogWriter.WriteThroughput.IdleDuration.Seconds()), testutil.ToFloat64(listener.logs.IdleDuration))
		assert.Equal(t, float64(metrics.Src.LogWriter.PendingBufferLen.Mean()), testutil.ToFloat64(listener.logs.PendingBufferLenMean))
		assert.Equal(t, float64(metrics.Src.LogWriter.SyncQueueLen.Mean()), testutil.ToFloat64(listener.logs.SyncQueueLenMean))
		assert.Equal(t, 1, testutil.CollectAndCount(listener.logs.FSyncLatency))
	})
}

// TestDualHistogram tests whether DualHistogram correctly registers a histogram with a different name,
// and verifies that the original histogram values are reflected.
// It ensures that only the histogram with the name "foo" is registered with Prometheus,
// and verifies this behavior.
func TestDualHistogram(t *testing.T) {
	src := prometheus.NewHistogram(prometheus.HistogramOpts{})
	wrapped := &dualHistogram{
		valueHist:      src,
		descrHistogram: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "foo"}),
	}
	prometheus.MustRegister(wrapped)
	src.Observe(1)
	// ensure that only foo is registered and src changes are seen.
	assert.Equal(t, 0, testutil.CollectAndCount(wrapped, "bar"))
	assert.Equal(t, 1, testutil.CollectAndCount(wrapped, "foo"))
}
