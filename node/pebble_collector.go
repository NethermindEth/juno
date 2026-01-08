package node

import (
	"strconv"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type pebbleSub string

const (
	pebbleNamespace = "pebble"
	numLevels       = 7 // Pebble has 7 levels (L0-L6)

	storage    pebbleSub = "storage"
	l0         pebbleSub = "l0"
	wal        pebbleSub = "wal"
	compact    pebbleSub = "compact"
	flush      pebbleSub = "flush"
	memtable   pebbleSub = "memtable"
	blockCache pebbleSub = "block_cache"
	tableCache pebbleSub = "table_cache"
	table      pebbleSub = "table"
	level      pebbleSub = "level"
)

// newPebbleCollector creates a new Prometheus collector for Pebble metrics.
func newPebbleCollector(nodeDB db.KeyValueStore) prometheus.CollectorFunc {
	pebbleDB, ok := nodeDB.Impl().(*pebble.DB)
	if !ok {
		return nil
	}

	// Storage metrics
	diskUsageBytes := storage.desc("disk_usage_bytes", "Total disk space used by the database")
	readAmp := storage.desc("read_amp", "Read amplification (total number of sublevels)")

	// L0 only metrics
	l0Sublevels := l0.desc("sublevels", "Number of L0 sublevels")
	l0TableBytesIn := l0.desc("table_bytes_in", "Bytes written to L0 (WAL bytes)")

	// WAL metrics
	walFiles := wal.desc("files", "Number of live WAL files")
	walSizeBytes := wal.desc("size_bytes", "Size of live WAL data in bytes")

	// Compaction metrics
	compactCount := compact.desc("count", "Number of compactions")
	compactNumInProgress := compact.desc("num_in_progress", "Number of in-progress compactions")
	compactInProgressBytes := compact.desc("in_progress_bytes", "Bytes being compacted")
	compactEstimatedDebtBytes := compact.desc("estimated_debt_bytes", "Estimated compaction debt")
	compactDurationSeconds := compact.desc("duration_seconds", "Compaction duration in seconds")
	compactFailedCount := compact.desc("failed_count", "Number of failed compactions")
	compactCancelledCount := compact.desc("cancelled_count", "Number of cancelled compactions")
	compactNumProblemSpans := compact.desc(
		"num_problem_spans",
		"Current count of problem spans blocking compactions",
	)

	// Memtable metrics
	memtableCount := memtable.desc("count", "Number of active memtables")
	memtableSizeBytes := memtable.desc("size_bytes", "Size of memtables in bytes")

	// Flush metrics
	flushCount := flush.desc("count", "Total number of flushes")
	flushNumInProgress := flush.desc("num_in_progress", "Number of flushes in progress")

	// Cache metrics
	blockCacheHits := blockCache.desc("hits", "Block cache hit count")
	blockCacheMisses := blockCache.desc("misses", "Block cache miss count")
	tableCacheHits := tableCache.desc("hits", "Table cache hit count")
	tableCacheMisses := tableCache.desc("misses", "Table cache miss count")

	// Iterator and Table metrics
	tableIters := table.desc("iters", "Number of open table iterators")
	tablesObsoleteCount := table.desc("obsolete_count", "Number of obsolete tables")
	tablesZombieCount := table.desc("zombie_count", "Number of zombie tables")

	// Per-level metrics
	levelTablesCount := level.desc("tables_count", "Number of tables per level", "level")
	levelSizeBytes := level.desc("size_bytes", "Size of tables per level in bytes", "level")
	levelScore := level.desc("score", "Compaction score per level", "level")
	levelFillFactor := level.desc("fill_factor", "Ratio between size and ideal size", "level")
	levelCompensatedFillFactor := level.desc(
		"compensated_fill_factor",
		"Compensated fill factor",
		"level",
	)
	levelWriteAmp := level.desc("write_amp", "Write amplification per level", "level")

	return func(ch chan<- prometheus.Metric) {
		m := pebbleDB.Metrics()

		sendAll(
			ch,
			// Storage metrics
			gauge(diskUsageBytes, float64(m.DiskSpaceUsage())),
			gauge(readAmp, float64(m.ReadAmp())),

			// L0 only metrics
			gauge(l0Sublevels, float64(m.Levels[0].Sublevels)),
			gauge(l0TableBytesIn, float64(m.Levels[0].TableBytesIn)),

			// WAL metrics
			gauge(walFiles, float64(m.WAL.Files)),
			gauge(walSizeBytes, float64(m.WAL.Size)),

			// Compaction metrics
			gauge(compactCount, float64(m.Compact.Count)),
			gauge(compactNumInProgress, float64(m.Compact.NumInProgress)),
			gauge(compactInProgressBytes, float64(m.Compact.InProgressBytes)),
			gauge(compactEstimatedDebtBytes, float64(m.Compact.EstimatedDebt)),
			gauge(compactDurationSeconds, m.Compact.Duration.Seconds()),
			gauge(compactFailedCount, float64(m.Compact.FailedCount)),
			gauge(compactCancelledCount, float64(m.Compact.CancelledCount)),
			gauge(compactNumProblemSpans, float64(m.Compact.NumProblemSpans)),

			// Memtable metrics
			gauge(memtableCount, float64(m.MemTable.Count)),
			gauge(memtableSizeBytes, float64(m.MemTable.Size)),

			// Flush metrics
			gauge(flushCount, float64(m.Flush.Count)),
			gauge(flushNumInProgress, float64(m.Flush.NumInProgress)),

			// Cache metrics
			gauge(blockCacheHits, float64(m.BlockCache.Hits)),
			gauge(blockCacheMisses, float64(m.BlockCache.Misses)),
			gauge(tableCacheHits, float64(m.FileCache.Hits)),
			gauge(tableCacheMisses, float64(m.FileCache.Misses)),

			// Iterator and Table metrics
			gauge(tableIters, float64(m.TableIters)),
			gauge(tablesObsoleteCount, float64(m.Table.ObsoleteCount)),
			gauge(tablesZombieCount, float64(m.Table.ZombieCount)),
		)

		// Per-level metrics
		for i := range numLevels {
			level := strconv.Itoa(i)
			sendAll(
				ch,
				gauge(levelTablesCount, float64(m.Levels[i].TablesCount), level),
				gauge(levelSizeBytes, float64(m.Levels[i].TablesSize), level),
				gauge(levelScore, m.Levels[i].Score, level),
				gauge(levelFillFactor, m.Levels[i].FillFactor, level),
				gauge(levelCompensatedFillFactor, m.Levels[i].CompensatedFillFactor, level),
				gauge(levelWriteAmp, m.Levels[i].WriteAmp(), level),
			)
		}
	}
}

// desc creates a new prometheus.Desc with the pebble namespace.
func (s pebbleSub) desc(name, help string, variableLabels ...string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(pebbleNamespace, string(s), name),
		help,
		variableLabels,
		nil,
	)
}

// gauge creates a gauge metric with the given descriptor and value.
func gauge(desc *prometheus.Desc, value float64, labelValues ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labelValues...)
}

// sendAll sends all metrics to the channel.
func sendAll(ch chan<- prometheus.Metric, metrics ...prometheus.Metric) {
	for _, m := range metrics {
		ch <- m
	}
}
