package pebble

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/metrics"
	"github.com/cockroachdb/pebble"
	"github.com/prometheus/client_golang/prometheus"
)

type metricsReporter interface {
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

const lvlsAmount = 7

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
		compactionDuration prometheus.Histogram           // Histogram for tracking compaction duration.
		compactionsDone    [lvlsAmount]prometheus.Counter // Counter for tracking amount of compactions per lvl.

		// 1:1 mapping of pebble.LevelMetrics.
		numFiles        [lvlsAmount]prometheus.Gauge
		size            [lvlsAmount]prometheus.Gauge
		score           [lvlsAmount]prometheus.Gauge
		bytesIn         [lvlsAmount]prometheus.Gauge
		bytesIngested   [lvlsAmount]prometheus.Gauge
		bytesMoved      [lvlsAmount]prometheus.Gauge
		bytesRead       [lvlsAmount]prometheus.Gauge
		bytesCompacted  [lvlsAmount]prometheus.Gauge
		bytesFlushed    [lvlsAmount]prometheus.Gauge
		tablesCompacted [lvlsAmount]prometheus.Counter
		tablesFlushed   [lvlsAmount]prometheus.Counter
		tablesIngested  [lvlsAmount]prometheus.Counter
		tablesMoved     [lvlsAmount]prometheus.Counter
	}
}

func newLevelsReporter() *levelsReporter {
	reporter := &levelsReporter{}
	reporter.metrics.compactionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "compaction_duration",
	})
	compactionsDone := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "compactions_done",
	}, []string{"lvl"})
	numFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "num_files",
	}, []string{"lvl"})
	size := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "size",
	}, []string{"lvl"})
	score := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "score",
	}, []string{"lvl"})
	bytesIn := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "bytes_in",
	}, []string{"lvl"})
	bytesIngested := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "bytes_ingested",
	}, []string{"lvl"})
	bytesMoved := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "bytes_moved",
	}, []string{"lvl"})
	bytesRead := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "bytes_read",
	}, []string{"lvl"})
	bytesCompacted := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "bytes_compacted",
	}, []string{"lvl"})
	bytesFlushed := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "bytes_flushed",
	}, []string{"lvl"})
	tablesCompacted := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "tables_compacted",
	}, []string{"lvl"})
	tablesFlushed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "tables_flushed",
	}, []string{"lvl"})
	tablesIngested := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "db",
		Subsystem: "lvl",
		Name:      "tables_ingested",
	}, []string{"lvl"})
	tablesMoved := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "db",
		Subsystem: "lvl",
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

func (reporter *levelsReporter) onCompBegin(ci pebble.CompactionInfo) {
	for _, input := range ci.Input {
		reporter.compLvls[input.Level].Add(1)
	}
}

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
		reporter.metrics.compactionDuration.Observe(compactionDuration.Seconds())
		reporter.metrics.compactionsDone[i].Add(float64(compactionsDone[i]))
		reporter.metrics.numFiles[i].Set(float64(lvl.NumFiles))
		reporter.metrics.size[i].Set(float64(lvl.Size))
		reporter.metrics.score[i].Set(lvl.Score)
		reporter.metrics.bytesIn[i].Set(float64(bytesIn[i]))
		reporter.metrics.bytesIngested[i].Set(float64(bytesIngested[i]))
		reporter.metrics.bytesMoved[i].Set(float64(bytesMoved[i]))
		reporter.metrics.bytesRead[i].Set(float64(bytesRead[i]))
		reporter.metrics.bytesCompacted[i].Set(float64(bytesCompacted[i]))
		reporter.metrics.bytesFlushed[i].Set(float64(bytesFlushed[i]))
		reporter.metrics.tablesCompacted[i].Add(float64(tablesCompacted[i]))
		reporter.metrics.tablesFlushed[i].Add(float64(tablesFlushed[i]))
		reporter.metrics.tablesIngested[i].Add(float64(tablesIngested[i]))
		reporter.metrics.tablesMoved[i].Add(float64(tablesMoved[i]))
	}
}

type diskReporter struct {
	// metrics
	diskUsage prometheus.Gauge
}

func newDiskReporter() *diskReporter {
	reporter := &diskReporter{}
	reporter.diskUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "disk",
		Name:      "usage",
	})
	metrics.MustRegister(reporter.diskUsage)
	return reporter
}

func (reporter *diskReporter) report(stats *pebble.Metrics) {
	reporter.diskUsage.Set(float64(stats.DiskSpaceUsage()))
}

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
		writeStall  prometheus.Histogram // Histogram for tracking delayed write durations due to compactions
		writeStalls prometheus.Counter   // Counter for tracking amount of delayed writes due to compactions
	}
}

func newStallReporter() *stallReporter {
	reporter := &stallReporter{}
	reporter.metrics.writeStall = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "db",
		Name:      "write_stall",
	})
	reporter.metrics.writeStalls = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "db",
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
	reporter.metrics.writeStall.Observe(float64(reporter.cache.writeStallDuration - cache.writeStallDuration))
	reporter.metrics.writeStalls.Add(float64(reporter.cache.writeStalls - cache.writeStalls))
}

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
	reporter := &compactReporter{}
	compactionsCounts := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "db",
		Subsystem: "compaction",
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
		Namespace: "db",
		Subsystem: "compaction",
		Name:      "estimated_debt",
	})
	reporter.metrics.InProgressBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "compaction",
		Name:      "in_progress_bytes",
	})
	reporter.metrics.NumInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "compaction",
		Name:      "num_in_progress",
	})
	reporter.metrics.MarkedFiles = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "db",
		Subsystem: "compaction",
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
