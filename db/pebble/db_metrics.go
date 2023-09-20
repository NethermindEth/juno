package pebble

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/metrics"
	"github.com/cockroachdb/pebble"
	"github.com/prometheus/client_golang/prometheus"
	gto "github.com/prometheus/client_model/go"
)

// metricsReporter is an interface designed for exporting Pebble database metrics.
type metricsReporter interface {
	// report collects and exposes important performance statistics like reads, writes and other relevant information.
	report(stats *pebble.Metrics)
}

// meter gathers metrics based on provided interval and reports them.
func (d *DB) meter(interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	// Gather metrics continuously at specified interval.
	for {
		<-timer.C
		timer.Reset(interval)
	}
}

const (
	lvlsAmount = 7
	namespace  = "db"
)

// levelsReporter reports about `pebble.Metrics.Levels` metrics and lvl compaction events.
type levelsReporter struct {
	compDuration atomic.Int64              // Total time taken for compactions.
	compLvls     [lvlsAmount]atomic.Uint64 // Total count of compactions done for lvls.

	// previous data
	cache struct {
		CompDuration time.Duration
		CompsAmount  [lvlsAmount]uint64
		Lvls         [lvlsAmount]pebble.LevelMetrics
	}

	metrics struct {
		compactionDuration prometheus.Counter             // Counter for tracking time spent in compaction.
		compactionsDone    [lvlsAmount]prometheus.Counter // Counter for tracking amount of compactions per lvl.

		// 1:1 mapping of pebble.LevelMetrics.
		numFiles        [lvlsAmount]prometheus.Gauge
		size            [lvlsAmount]prometheus.Gauge
		score           [lvlsAmount]prometheus.Gauge
		bytesIn         [lvlsAmount]prometheus.Counter
		bytesIngested   [lvlsAmount]prometheus.Counter
		bytesMoved      [lvlsAmount]prometheus.Counter
		bytesRead       [lvlsAmount]prometheus.Counter
		bytesCompacted  [lvlsAmount]prometheus.Counter
		bytesFlushed    [lvlsAmount]prometheus.Counter
		tablesCompacted [lvlsAmount]prometheus.Counter
		tablesFlushed   [lvlsAmount]prometheus.Counter
		tablesIngested  [lvlsAmount]prometheus.Counter
		tablesMoved     [lvlsAmount]prometheus.Counter
	}
}

func newLevelsReporter() *levelsReporter {
	const subsystem = "lvl"
	reporter := &levelsReporter{}
	reporter.metrics.compactionDuration = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "compaction_duration",
	})
	compactionsDone := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "compactions_done",
	}, []string{"lvl"})
	numFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "num_files",
	}, []string{"lvl"})
	size := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "size",
	}, []string{"lvl"})
	score := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "score",
	}, []string{"lvl"})
	bytesIn := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "bytes_in",
	}, []string{"lvl"})
	bytesIngested := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "bytes_ingested",
	}, []string{"lvl"})
	bytesMoved := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "bytes_moved",
	}, []string{"lvl"})
	bytesRead := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "bytes_read",
	}, []string{"lvl"})
	bytesCompacted := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "bytes_compacted",
	}, []string{"lvl"})
	bytesFlushed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "bytes_flushed",
	}, []string{"lvl"})
	tablesCompacted := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "tables_compacted",
	}, []string{"lvl"})
	tablesFlushed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "tables_flushed",
	}, []string{"lvl"})
	tablesIngested := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "tables_ingested",
	}, []string{"lvl"})
	tablesMoved := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "tables_moved",
	}, []string{"lvl"})
	for i := 0; i < lvlsAmount; i++ {
		label := strconv.Itoa(i)
		reporter.metrics.compactionsDone[i] = compactionsDone.WithLabelValues(label)
		reporter.metrics.numFiles[i] = numFiles.WithLabelValues(label)
		reporter.metrics.size[i] = size.WithLabelValues(label)
		reporter.metrics.score[i] = score.WithLabelValues(label)
		reporter.metrics.bytesIn[i] = bytesIn.WithLabelValues(label)
		reporter.metrics.bytesIngested[i] = bytesIngested.WithLabelValues(label)
		reporter.metrics.bytesMoved[i] = bytesMoved.WithLabelValues(label)
		reporter.metrics.bytesRead[i] = bytesRead.WithLabelValues(label)
		reporter.metrics.bytesCompacted[i] = bytesCompacted.WithLabelValues(label)
		reporter.metrics.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		reporter.metrics.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		reporter.metrics.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		reporter.metrics.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		reporter.metrics.tablesCompacted[i] = tablesCompacted.WithLabelValues(label)
		reporter.metrics.tablesFlushed[i] = tablesFlushed.WithLabelValues(label)
		reporter.metrics.tablesIngested[i] = tablesIngested.WithLabelValues(label)
		reporter.metrics.tablesMoved[i] = tablesMoved.WithLabelValues(label)
	}
	metrics.MustRegister(
		reporter.metrics.compactionDuration,
		compactionsDone,
		numFiles,
		size,
		score,
		bytesIn,
		bytesIngested,
		bytesMoved,
		bytesRead,
		bytesCompacted,
		bytesFlushed,
		tablesCompacted,
		tablesFlushed,
		tablesIngested,
		tablesMoved,
	)
	return reporter
}

//nolint:gocritic
func (reporter *levelsReporter) onCompBegin(ci pebble.CompactionInfo) {
	for _, input := range ci.Input {
		reporter.compLvls[input.Level].Add(1)
	}
}

//nolint:gocritic
func (reporter *levelsReporter) onCompEnd(ci pebble.CompactionInfo) {
	reporter.compDuration.Add(int64(ci.TotalDuration))
}

func (reporter *levelsReporter) report(stats *pebble.Metrics) {
	// swap cache
	levels := stats.Levels
	cache := reporter.cache
	reporter.cache.Lvls = levels

	// final stats
	var (
		compactionDuration time.Duration
		compactionsDone    [lvlsAmount]uint64
		bytesIn            [lvlsAmount]uint64
		bytesIngested      [lvlsAmount]uint64
		bytesMoved         [lvlsAmount]uint64
		bytesRead          [lvlsAmount]uint64
		bytesCompacted     [lvlsAmount]uint64
		bytesFlushed       [lvlsAmount]uint64
		tablesCompacted    [lvlsAmount]uint64
		tablesFlushed      [lvlsAmount]uint64
		tablesIngested     [lvlsAmount]uint64
		tablesMoved        [lvlsAmount]uint64
	)

	// calculate finals
	for i := 0; i < lvlsAmount; i++ {
		comps := reporter.compLvls[i].Load()
		compDuration := time.Duration(reporter.compDuration.Load())

		compactionDuration = compDuration - cache.CompDuration
		compactionsDone[i] = comps - cache.CompsAmount[i]
		bytesIn[i] = levels[i].BytesIn - cache.Lvls[i].BytesIn
		bytesIngested[i] = levels[i].BytesIngested - cache.Lvls[i].BytesIngested
		bytesMoved[i] = levels[i].BytesMoved - cache.Lvls[i].BytesMoved
		bytesRead[i] = levels[i].BytesRead - cache.Lvls[i].BytesRead
		bytesCompacted[i] = levels[i].BytesCompacted - cache.Lvls[i].BytesCompacted
		bytesFlushed[i] = levels[i].BytesFlushed - cache.Lvls[i].BytesFlushed
		tablesCompacted[i] = levels[i].TablesCompacted - cache.Lvls[i].TablesCompacted
		tablesFlushed[i] = levels[i].TablesFlushed - cache.Lvls[i].TablesFlushed
		tablesIngested[i] = levels[i].TablesIngested - cache.Lvls[i].TablesIngested
		tablesMoved[i] = levels[i].TablesMoved - cache.Lvls[i].TablesMoved

		// update cache
		reporter.cache.CompsAmount[i] = comps
		reporter.cache.CompDuration = compDuration
	}

	// report
	for i := 0; i < lvlsAmount; i++ {
		lvl := levels[i]
		reporter.metrics.compactionDuration.Add(compactionDuration.Seconds())
		reporter.metrics.compactionsDone[i].Add(float64(compactionsDone[i]))
		reporter.metrics.numFiles[i].Set(float64(lvl.NumFiles))
		reporter.metrics.size[i].Set(float64(lvl.Size))
		reporter.metrics.score[i].Set(lvl.Score)
		reporter.metrics.bytesIn[i].Add(float64(bytesIn[i]))
		reporter.metrics.bytesIngested[i].Add(float64(bytesIngested[i]))
		reporter.metrics.bytesMoved[i].Add(float64(bytesMoved[i]))
		reporter.metrics.bytesRead[i].Add(float64(bytesRead[i]))
		reporter.metrics.bytesCompacted[i].Add(float64(bytesCompacted[i]))
		reporter.metrics.bytesFlushed[i].Add(float64(bytesFlushed[i]))
		reporter.metrics.tablesCompacted[i].Add(float64(tablesCompacted[i]))
		reporter.metrics.tablesFlushed[i].Add(float64(tablesFlushed[i]))
		reporter.metrics.tablesIngested[i].Add(float64(tablesIngested[i]))
		reporter.metrics.tablesMoved[i].Add(float64(tablesMoved[i]))
	}
}

// diskReporter reports about disk usage.
type diskReporter struct {
	// metrics
	diskUsage prometheus.Gauge
}

func newDiskReporter() *diskReporter {
	const subsystem = "disk"
	reporter := &diskReporter{}
	reporter.diskUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "usage",
	})
	metrics.MustRegister(reporter.diskUsage)
	return reporter
}

func (reporter *diskReporter) report(stats *pebble.Metrics) {
	reporter.diskUsage.Set(float64(stats.DiskSpaceUsage()))
}

// stallReporter reports about events of stall.
type stallReporter struct {
	writeStallStart    time.Time     // timestamp of last stall write
	writeStallDuration atomic.Int64  // duration of stalled writes
	writeStalls        atomic.Uint64 // amount of stalled writes

	// previous data
	cache struct {
		writeStallDuration time.Duration
		writeStalls        uint64
	}

	metrics struct {
		writeStall  prometheus.Counter // Histogram for tracking amount of delayed time due to compactions.
		writeStalls prometheus.Counter // Counter for tracking amount of delayed writes due to compactions
	}
}

func newStallReporter() *stallReporter {
	reporter := &stallReporter{}
	reporter.metrics.writeStall = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "write_stall",
	})
	reporter.metrics.writeStalls = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "write_stalls",
	})
	metrics.MustRegister(
		reporter.metrics.writeStall,
		reporter.metrics.writeStalls,
	)
	return reporter
}

func (reporter *stallReporter) onWriteStallStart(_ pebble.WriteStallBeginInfo) {
	reporter.writeStallStart = time.Now()
	reporter.writeStalls.Add(1)
}

func (reporter *stallReporter) onWriteStallEnd() {
	reporter.writeStallDuration.Add(int64(time.Since(reporter.writeStallStart)))
}

func (reporter *stallReporter) report(_ *pebble.Metrics) {
	cache := reporter.cache
	reporter.cache.writeStalls = reporter.writeStalls.Load()
	reporter.cache.writeStallDuration = time.Duration(reporter.writeStallDuration.Load())
	reporter.metrics.writeStall.Add(float64(reporter.cache.writeStallDuration - cache.writeStallDuration))
	reporter.metrics.writeStalls.Add(float64(reporter.cache.writeStalls - cache.writeStalls))
}

// compactReporter reports about `pebble.Metrics.Compact` metrics.
type compactReporter struct {
	// previous data
	cache struct {
		DefaultCount     int64
		DeleteOnlyCount  int64
		ElisionOnlyCount int64
		MoveCount        int64
		ReadCount        int64
		RewriteCount     int64
		MultiLevelCount  int64
	}

	metrics struct {
		DefaultCount     prometheus.Counter
		DeleteOnlyCount  prometheus.Counter
		ElisionOnlyCount prometheus.Counter
		MoveCount        prometheus.Counter
		ReadCount        prometheus.Counter
		RewriteCount     prometheus.Counter
		MultiLevelCount  prometheus.Counter
		EstimatedDebt    prometheus.Gauge
		InProgressBytes  prometheus.Gauge
		NumInProgress    prometheus.Gauge
		MarkedFiles      prometheus.Gauge
	}
}

func newCompactReporter() *compactReporter {
	const subsystem = "compaction"
	reporter := &compactReporter{}
	compactionsCounts := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "amount",
	}, []string{"type"})
	reporter.metrics.DefaultCount = compactionsCounts.WithLabelValues("default")
	reporter.metrics.DeleteOnlyCount = compactionsCounts.WithLabelValues("delete")
	reporter.metrics.ElisionOnlyCount = compactionsCounts.WithLabelValues("elision")
	reporter.metrics.MoveCount = compactionsCounts.WithLabelValues("move")
	reporter.metrics.ReadCount = compactionsCounts.WithLabelValues("read")
	reporter.metrics.RewriteCount = compactionsCounts.WithLabelValues("rewrite")
	reporter.metrics.MultiLevelCount = compactionsCounts.WithLabelValues("multi_level")
	reporter.metrics.EstimatedDebt = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "estimated_debt",
	})
	reporter.metrics.InProgressBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "in_progress_bytes",
	})
	reporter.metrics.NumInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "num_in_progress",
	})
	reporter.metrics.MarkedFiles = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "marked_files",
	})
	metrics.MustRegister(
		compactionsCounts,
		reporter.metrics.EstimatedDebt,
		reporter.metrics.InProgressBytes,
		reporter.metrics.NumInProgress,
		reporter.metrics.MarkedFiles,
	)
	return reporter
}

func (reporter *compactReporter) report(stats *pebble.Metrics) {
	cache := reporter.cache
	reporter.cache.DefaultCount = stats.Compact.DefaultCount
	reporter.cache.DeleteOnlyCount = stats.Compact.DeleteOnlyCount
	reporter.cache.ElisionOnlyCount = stats.Compact.ElisionOnlyCount
	reporter.cache.MoveCount = stats.Compact.MoveCount
	reporter.cache.ReadCount = stats.Compact.ReadCount
	reporter.cache.RewriteCount = stats.Compact.RewriteCount
	reporter.cache.MultiLevelCount = stats.Compact.MultiLevelCount

	reporter.metrics.DefaultCount.Add(float64(reporter.cache.DefaultCount - cache.DefaultCount))
	reporter.metrics.DeleteOnlyCount.Add(float64(reporter.cache.DeleteOnlyCount - cache.DeleteOnlyCount))
	reporter.metrics.ElisionOnlyCount.Add(float64(reporter.cache.ElisionOnlyCount - cache.ElisionOnlyCount))
	reporter.metrics.MoveCount.Add(float64(reporter.cache.MoveCount - cache.MoveCount))
	reporter.metrics.ReadCount.Add(float64(reporter.cache.ReadCount - cache.ReadCount))
	reporter.metrics.RewriteCount.Add(float64(reporter.cache.RewriteCount - cache.RewriteCount))
	reporter.metrics.MultiLevelCount.Add(float64(reporter.cache.MultiLevelCount - cache.MultiLevelCount))
	reporter.metrics.EstimatedDebt.Set(float64(stats.Compact.EstimatedDebt))
	reporter.metrics.InProgressBytes.Set(float64(stats.Compact.InProgressBytes))
	reporter.metrics.NumInProgress.Set(float64(stats.Compact.NumInProgress))
	reporter.metrics.MarkedFiles.Set(float64(stats.Compact.MarkedFiles))
}

// cacheReporter reports about `pebble.Metrics.BlockCache` metrics.
type cacheReporter struct {
	cache struct {
		Hits   uint64
		Misses uint64
	}
	metrics struct {
		Size   prometheus.Gauge
		Count  prometheus.Gauge
		Hits   prometheus.Counter
		Misses prometheus.Counter
	}
}

func newCacheReporter() *cacheReporter {
	const subsystem = "cache"
	reporter := &cacheReporter{}
	reporter.metrics.Size = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "size",
	})
	reporter.metrics.Count = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "count",
	})
	hitCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "hits",
	}, []string{"succesfull"})
	reporter.metrics.Hits = hitCounter.WithLabelValues("true")
	reporter.metrics.Misses = hitCounter.WithLabelValues("false")
	metrics.MustRegister(
		reporter.metrics.Size,
		reporter.metrics.Count,
		hitCounter,
	)
	return reporter
}

func (reporter *cacheReporter) report(stats *pebble.Metrics) {
	cache := reporter.cache

	reporter.cache.Hits = uint64(stats.BlockCache.Hits)
	reporter.cache.Misses = uint64(stats.BlockCache.Misses)

	reporter.metrics.Size.Set(float64(stats.BlockCache.Size))
	reporter.metrics.Count.Set(float64(stats.BlockCache.Count))
	reporter.metrics.Hits.Add(float64(reporter.cache.Hits - cache.Hits))
	reporter.metrics.Misses.Add(float64(reporter.cache.Misses - cache.Misses))
}

// flushReporter reports about `pebble.Metrics.Flush` metrics.
type flushReporter struct {
	// previous data
	cache struct {
		AsIngestCount      uint64
		AsIngestTableCount uint64
		AsIngestBytes      uint64
		BytesProcessed     uint64
		Count              uint64
		WorkDuration       time.Duration
		IdleDuration       time.Duration
	}

	metrics struct {
		Count              prometheus.Counter
		AsIngestCount      prometheus.Counter
		AsIngestTableCount prometheus.Counter
		AsIngestBytes      prometheus.Counter
		BytesProcessed     prometheus.Counter
		NumInProgress      prometheus.Gauge
		WorkDuration       prometheus.Counter
		IdleDuration       prometheus.Counter
	}
}

func newFlushReporter() *flushReporter {
	const subsystem = "flush"
	reporter := &flushReporter{}
	reporter.metrics.Count = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "amount",
	})
	reporter.metrics.AsIngestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "ingests",
	})
	reporter.metrics.AsIngestTableCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "ingest_tables",
	})
	reporter.metrics.AsIngestBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "ingest_bytes",
	})
	reporter.metrics.BytesProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "bytes_processed",
	})
	reporter.metrics.NumInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "in_progress",
	})
	workCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "work",
	}, []string{"state"})
	reporter.metrics.WorkDuration = workCounter.WithLabelValues("work")
	reporter.metrics.IdleDuration = workCounter.WithLabelValues("idle")
	metrics.MustRegister(
		reporter.metrics.Count,
		reporter.metrics.AsIngestCount,
		reporter.metrics.AsIngestTableCount,
		reporter.metrics.AsIngestBytes,
		reporter.metrics.NumInProgress,
		reporter.metrics.BytesProcessed,
		workCounter,
	)
	return reporter
}

func (reporter *flushReporter) report(stats *pebble.Metrics) {
	cache := reporter.cache

	reporter.cache.Count = uint64(stats.Flush.Count)
	reporter.cache.AsIngestCount = stats.Flush.AsIngestCount
	reporter.cache.AsIngestTableCount = stats.Flush.AsIngestTableCount
	reporter.cache.AsIngestBytes = stats.Flush.AsIngestBytes
	reporter.cache.BytesProcessed = uint64(stats.Flush.WriteThroughput.Bytes)
	reporter.cache.IdleDuration = stats.Flush.WriteThroughput.IdleDuration
	reporter.cache.WorkDuration = stats.Flush.WriteThroughput.WorkDuration

	reporter.metrics.Count.Add(float64(reporter.cache.Count - cache.Count))
	reporter.metrics.AsIngestCount.Add(float64(reporter.cache.AsIngestCount - cache.AsIngestCount))
	reporter.metrics.AsIngestTableCount.Add(float64(reporter.cache.AsIngestTableCount - cache.AsIngestTableCount))
	reporter.metrics.AsIngestBytes.Add(float64(reporter.cache.AsIngestBytes - cache.AsIngestBytes))
	reporter.metrics.BytesProcessed.Add(float64(reporter.cache.BytesProcessed - cache.BytesProcessed))
	reporter.metrics.IdleDuration.Add((reporter.cache.IdleDuration - cache.IdleDuration).Seconds())
	reporter.metrics.WorkDuration.Add((reporter.cache.WorkDuration - cache.WorkDuration).Seconds())
}

// filterReporter reports about `pebble.Metrics.Filter` metrics.
type filterReporter struct {
	// previous data
	cache struct {
		Hits   uint64
		Misses uint64
	}

	metrics struct {
		Hits   prometheus.Counter
		Misses prometheus.Counter
	}
}

func newFilterReporter() *filterReporter {
	const subsystem = "filter"
	reporter := &filterReporter{}
	filterCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "hits",
	}, []string{"succesfull"})
	reporter.metrics.Hits = filterCounter.WithLabelValues("true")
	reporter.metrics.Misses = filterCounter.WithLabelValues("false")
	metrics.MustRegister(filterCounter)
	return reporter
}

func (reporter *filterReporter) report(stats *pebble.Metrics) {
	cache := reporter.cache

	reporter.cache.Hits = uint64(stats.Filter.Hits)
	reporter.cache.Misses = uint64(stats.Filter.Misses)

	reporter.metrics.Hits.Add(float64(reporter.cache.Hits - cache.Hits))
	reporter.metrics.Misses.Add(float64(reporter.cache.Misses - cache.Misses))
}

// memtableReporter reports about `pebble.Metrics.MemTable` metrics.
type memtableReporter struct {
	metrics struct {
		Size       prometheus.Gauge
		Count      prometheus.Gauge
		ZombieSize prometheus.Gauge
		Zombies    prometheus.Gauge
	}
}

//nolint:dupl
func newMemtableReporter() *memtableReporter {
	const subsystem = "memtable"
	reporter := &memtableReporter{}
	reporter.metrics.Size = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "size",
	})
	reporter.metrics.Count = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "amount",
	})
	reporter.metrics.ZombieSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "zombie_size",
	})
	reporter.metrics.Zombies = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "zombies",
	})
	metrics.MustRegister(
		reporter.metrics.Size,
		reporter.metrics.Count,
		reporter.metrics.ZombieSize,
		reporter.metrics.Zombies,
	)
	return reporter
}

func (reporter *memtableReporter) report(stats *pebble.Metrics) {
	reporter.metrics.Size.Set(float64(stats.MemTable.Size))
	reporter.metrics.Count.Set(float64(stats.MemTable.Count))
	reporter.metrics.ZombieSize.Set(float64(stats.MemTable.ZombieSize))
	reporter.metrics.Zombies.Set(float64(stats.MemTable.ZombieCount))
}

// keysReporter reports about `pebble.Metrics.Keys` metrics.
type keysReporter struct {
	metrics struct {
		RangeKeySets prometheus.Gauge
		Tombstones   prometheus.Gauge
	}
}

func newKeysReporter() *keysReporter {
	const subsystem = "keys"
	reporter := &keysReporter{}
	reporter.metrics.RangeKeySets = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "range_key_sets",
	})
	reporter.metrics.Tombstones = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "tombstones",
	})
	metrics.MustRegister(
		reporter.metrics.RangeKeySets,
		reporter.metrics.Tombstones,
	)
	return reporter
}

func (reporter *keysReporter) report(stats *pebble.Metrics) {
	reporter.metrics.RangeKeySets.Set(float64(stats.Keys.RangeKeySetsCount))
	reporter.metrics.Tombstones.Set(float64(stats.Keys.TombstoneCount))
}

// snapshotsReporter reports about `pebble.Metrics.Snapshots` metrics.
type snapshotsReporter struct {
	cache struct {
		PinnedKeys uint64
		PinnedSize uint64
	}

	metrics struct {
		Count          prometheus.Gauge
		EarliestSeqNum prometheus.Gauge
		PinnedKeys     prometheus.Counter
		PinnedSize     prometheus.Counter
	}
}

//nolint:dupl
func newSnapshotReporter() *snapshotsReporter {
	const subsystem = "snapshots"
	reporter := &snapshotsReporter{}
	reporter.metrics.Count = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "amount",
	})
	reporter.metrics.EarliestSeqNum = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "earliest_seq_num",
	})
	reporter.metrics.PinnedKeys = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "pinned_keys",
	})
	reporter.metrics.PinnedSize = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "pinned_size",
	})
	metrics.MustRegister(
		reporter.metrics.Count,
		reporter.metrics.EarliestSeqNum,
		reporter.metrics.PinnedKeys,
		reporter.metrics.PinnedSize,
	)
	return reporter
}

func (reporter *snapshotsReporter) report(stats *pebble.Metrics) {
	cache := reporter.cache

	reporter.cache.PinnedKeys = stats.Snapshots.PinnedKeys
	reporter.cache.PinnedSize = stats.Snapshots.PinnedSize

	reporter.metrics.Count.Set(float64(stats.Snapshots.Count))
	reporter.metrics.EarliestSeqNum.Set(float64(stats.Snapshots.EarliestSeqNum))
	reporter.metrics.PinnedKeys.Add(float64(reporter.cache.PinnedKeys - cache.PinnedKeys))
	reporter.metrics.PinnedSize.Add(float64(reporter.cache.PinnedSize - cache.PinnedSize))
}

// tableReporter reports about `pebble.Metrics.Table`, `pebble.Metrics.TableCache` and `pebble.Metrics.TableIters` metrics.
type tableReporter struct {
	metrics struct {
		// Table metrics
		ObsoleteSize  prometheus.Gauge
		ObsoleteCount prometheus.Gauge
		ZombieSize    prometheus.Gauge
		ZombieCount   prometheus.Gauge

		// TableCache metrics
		Size   prometheus.Gauge
		Count  prometheus.Gauge
		Hits   prometheus.Gauge
		Misses prometheus.Gauge

		// TableIters
		Iters prometheus.Gauge
	}
}

func newTableReporter() *tableReporter {
	const subsystem = "table"
	reporter := &tableReporter{}
	reporter.metrics.ObsoleteSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "obsolete_size",
	})
	reporter.metrics.ObsoleteCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "obsolete",
	})
	reporter.metrics.ZombieSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "zombie_size",
	})
	reporter.metrics.ZombieCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "zombies",
	})
	reporter.metrics.Size = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "cache_size",
	})
	reporter.metrics.Count = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "cache_count",
	})
	hitsGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "cache_hits",
	}, []string{"succesfull"})
	reporter.metrics.Hits = hitsGauge.WithLabelValues("true")
	reporter.metrics.Misses = hitsGauge.WithLabelValues("false")
	reporter.metrics.Iters = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "iters",
	})
	metrics.MustRegister(
		reporter.metrics.ObsoleteSize,
		reporter.metrics.ObsoleteCount,
		reporter.metrics.ZombieSize,
		reporter.metrics.ZombieCount,
		reporter.metrics.Size,
		reporter.metrics.Count,
		hitsGauge,
		reporter.metrics.Iters,
	)
	return reporter
}

func (reporter *tableReporter) report(stats *pebble.Metrics) {
	reporter.metrics.ObsoleteSize.Set(float64(stats.Table.ObsoleteSize))
	reporter.metrics.ObsoleteCount.Set(float64(stats.Table.ObsoleteCount))
	reporter.metrics.ZombieSize.Set(float64(stats.Table.ZombieSize))
	reporter.metrics.ZombieCount.Set(float64(stats.Table.ZombieCount))
	reporter.metrics.Size.Set(float64(stats.Table.ObsoleteSize))
	reporter.metrics.Count.Set(float64(stats.Table.ObsoleteCount))
	reporter.metrics.Hits.Set(float64(stats.Table.ZombieSize))
	reporter.metrics.Misses.Set(float64(stats.Table.ZombieCount))
	reporter.metrics.Iters.Set(float64(stats.TableIters))
}

// walReporter reports about `pebble.Metrics.Wal` metrics
type walReporter struct {
	cache struct {
		BytesIn      uint64
		BytesWritten uint64
	}

	metrics struct {
		Files                prometheus.Gauge
		ObsoleteFiles        prometheus.Gauge
		ObsoletePhysicalSize prometheus.Gauge
		Size                 prometheus.Gauge
		PhysicalSize         prometheus.Gauge
		BytesIn              prometheus.Counter
		BytesWritten         prometheus.Counter
	}
}

func newWalReporter() *walReporter {
	const subsystem = "wal"
	reporter := &walReporter{}
	reporter.metrics.Files = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "files",
	})
	reporter.metrics.ObsoleteFiles = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "obsolete_files",
	})
	reporter.metrics.ObsoletePhysicalSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "obsolete_physical_size",
	})
	reporter.metrics.Size = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "size",
	})
	reporter.metrics.PhysicalSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "physical_size",
	})
	reporter.metrics.BytesIn = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "bytes_in",
	})
	reporter.metrics.BytesWritten = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "bytes_written",
	})
	metrics.MustRegister(
		reporter.metrics.Files,
		reporter.metrics.ObsoleteFiles,
		reporter.metrics.ObsoletePhysicalSize,
		reporter.metrics.Size,
		reporter.metrics.PhysicalSize,
		reporter.metrics.BytesIn,
		reporter.metrics.BytesWritten,
	)
	return reporter
}

func (reporter *walReporter) report(stats *pebble.Metrics) {
	cache := reporter.cache

	reporter.cache.BytesIn = stats.WAL.BytesIn
	reporter.cache.BytesWritten = stats.WAL.BytesWritten

	reporter.metrics.Files.Set(float64(stats.WAL.Files))
	reporter.metrics.ObsoleteFiles.Set(float64(stats.WAL.ObsoleteFiles))
	reporter.metrics.ObsoletePhysicalSize.Set(float64(stats.WAL.ObsoletePhysicalSize))
	reporter.metrics.Size.Set(float64(stats.WAL.Size))
	reporter.metrics.PhysicalSize.Set(float64(stats.WAL.PhysicalSize))
	reporter.metrics.BytesIn.Add(float64(reporter.cache.BytesIn - cache.BytesIn))
	reporter.metrics.BytesWritten.Add(float64(reporter.cache.BytesWritten - cache.BytesWritten))
}

// logsReporter reports about `pebble.Metrics.LogWriter` metrics
type logsReporter struct {
	cache struct {
		Bytes        uint64
		IdleDuration time.Duration
		WorkDuration time.Duration
	}
	metrics struct {
		// FSyncLatency is managed by the pebble itself
		FSyncLatency prometheus.Histogram

		// WriteThroughput metrics
		Bytes        prometheus.Counter
		WorkDuration prometheus.Counter
		IdleDuration prometheus.Counter

		// PendingBufferLen metric
		PendingBufferLenMean prometheus.Gauge
		// SyncQueueLen metric
		SyncQueueLenMean prometheus.Gauge
	}

	once sync.Once
}

func newLogsReporter() *logsReporter {
	const subsystem = "logs"
	reporter := &logsReporter{}

	reporter.metrics.Bytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "write_throughput_bytes",
	})
	workCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "write_throughput_work",
	}, []string{"state"})
	reporter.metrics.WorkDuration = workCounter.WithLabelValues("work")
	reporter.metrics.IdleDuration = workCounter.WithLabelValues("idle")
	reporter.metrics.PendingBufferLenMean = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "pending_buffer_len",
	})
	reporter.metrics.SyncQueueLenMean = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "sync_queue_len",
	})
	metrics.MustRegister(
		reporter.metrics.Bytes,
		workCounter,
		reporter.metrics.PendingBufferLenMean,
		reporter.metrics.SyncQueueLenMean,
	)
	return reporter
}

func (reporter *logsReporter) report(stats *pebble.Metrics) {
	// The 'pebble.Metrics.LogWriter.FsyncLatency' metric lacks a valid description for registration.
	// Furthermore, it doesn't provide a direct method for manual data retrieval.
	// To address these issues, we encapsulate it within another histogram while providing
	// a meaningful description that aligns with our specific use case.
	reporter.once.Do(func() {
		reporter.registerFSyncHistogram(stats.LogWriter.FsyncLatency)
	})

	cache := reporter.cache
	reporter.cache.Bytes = uint64(stats.LogWriter.WriteThroughput.Bytes)
	reporter.cache.WorkDuration = stats.LogWriter.WriteThroughput.WorkDuration
	reporter.cache.IdleDuration = stats.LogWriter.WriteThroughput.IdleDuration

	reporter.metrics.Bytes.Add(float64(reporter.cache.Bytes - cache.Bytes))
	reporter.metrics.WorkDuration.Add((reporter.cache.WorkDuration - cache.WorkDuration).Seconds())
	reporter.metrics.IdleDuration.Add((reporter.cache.IdleDuration - cache.IdleDuration).Seconds())
	reporter.metrics.PendingBufferLenMean.Set(stats.LogWriter.PendingBufferLen.Mean())
	reporter.metrics.SyncQueueLenMean.Set(stats.LogWriter.SyncQueueLen.Mean())
}

func (reporter *logsReporter) registerFSyncHistogram(hist prometheus.Histogram) {
	wrapped := &dualHistogram{}
	wrapped.valueHist = hist
	wrapped.descrHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "logs",
		Name:      "fsync_latency",
	})
	reporter.metrics.FSyncLatency = wrapped
	metrics.MustRegister(reporter.metrics.FSyncLatency)
}

// dualHistogram is a histogram wrapper for reporting histogram under different description.
type dualHistogram struct {
	// valueHist represents a Prometheus Histogram for tracking metric values.
	valueHist prometheus.Histogram
	// descrHistogram represents a Prometheus Histogram for description.
	descrHistogram prometheus.Histogram
}

func (hist *dualHistogram) Observe(v float64) { hist.valueHist.Observe(v) }

func (hist *dualHistogram) Describe(ch chan<- *prometheus.Desc) { hist.descrHistogram.Describe(ch) }

func (hist *dualHistogram) Desc() *prometheus.Desc { return hist.descrHistogram.Desc() }

func (hist *dualHistogram) Write(metric *gto.Metric) error { return hist.valueHist.Write(metric) }

func (hist *dualHistogram) Collect(ch chan<- prometheus.Metric) { ch <- hist }
