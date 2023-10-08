package node

import (
	"strconv"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/sync"
	"github.com/cockroachdb/pebble"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	dbLvlsAmount = 7
	dbNamespace  = "db"
)

// pebbleListener listens for pebble metrics.
type pebbleListener struct {
	levels    *levelsListener     // Listener for level-specific metrics.
	comp      *compactionListener // Listener for compaction-specific metrics.
	cache     *cacheListener      // Listener for cache-specific metrics.
	flush     *flushListener      // Listener for flush-specific metrics.
	filter    *filterListener     // Listener for filter-specific metrics.
	memtable  *memtableListener   // Listener for memtable-specific metrics.
	keys      *keysListener       // Listener for keys-specific metrics.
	snapshots *snapshotsListener  // Listener for snapshots-specific metrics.
}

// newPebbleListener creates and returns a new pebbleListener instance with initialized listeners.
func newPebbleListener() *pebbleListener {
	return &pebbleListener{
		levels:    newLevelsListener(),
		comp:      newCompactionListener(),
		cache:     newCacheListener(),
		flush:     newFlushListener(),
		filter:    newFilterListener(),
		memtable:  newMemtableListener(),
		keys:      newKeysListener(),
		snapshots: newSnapshotListener(),
	}
}

// gather collects and updates metrics from a Pebble database.
//
// This method delegates the collection of various metrics to their respective listeners.
func (listener *pebbleListener) gather(metrics *db.PebbleMetrics) {
	listener.levels.gather(metrics)    // gather level-specific metrics.
	listener.comp.gather(metrics)      // gather compaction-specific metrics.
	listener.cache.gather(metrics)     // gather cache-specific metrics.
	listener.flush.gather(metrics)     // gather flush-specific metrics.
	listener.filter.gather(metrics)    // gather filter-specific metrics.
	listener.memtable.gather(metrics)  // gather memtable-specific metrics.
	listener.keys.gather(metrics)      // gather keys-specific metrics.
	listener.snapshots.gather(metrics) // gather snapshots-specific metrics.
}

// levelsListener listens for pebble levels metrics.
type levelsListener struct {
	// cache stores previous data in order to calculate delta.
	cache struct {
		Lvls [dbLvlsAmount]pebble.LevelMetrics
	}

	// metrics
	numFiles        [dbLvlsAmount]prometheus.Gauge
	size            [dbLvlsAmount]prometheus.Gauge
	score           [dbLvlsAmount]prometheus.Gauge
	bytesIn         [dbLvlsAmount]prometheus.Counter
	bytesIngested   [dbLvlsAmount]prometheus.Counter
	bytesMoved      [dbLvlsAmount]prometheus.Counter
	bytesRead       [dbLvlsAmount]prometheus.Counter
	bytesCompacted  [dbLvlsAmount]prometheus.Counter
	bytesFlushed    [dbLvlsAmount]prometheus.Counter
	tablesCompacted [dbLvlsAmount]prometheus.Counter
	tablesFlushed   [dbLvlsAmount]prometheus.Counter
	tablesIngested  [dbLvlsAmount]prometheus.Counter
	tablesMoved     [dbLvlsAmount]prometheus.Counter
}

// newLevelsListener creates and returns a new levelsListener instance with setup prometheus metrics.
func newLevelsListener() *levelsListener {
	const subsystem = "lvl"
	listener := &levelsListener{}
	numFiles := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "num_files",
	}, []string{"lvl"})
	size := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "size",
	}, []string{"lvl"})
	score := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "score",
	}, []string{"lvl"})
	bytesIn := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_in",
	}, []string{"lvl"})
	bytesIngested := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_ingested",
	}, []string{"lvl"})
	bytesMoved := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_moved",
	}, []string{"lvl"})
	bytesRead := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_read",
	}, []string{"lvl"})
	bytesCompacted := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_compacted",
	}, []string{"lvl"})
	bytesFlushed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_flushed",
	}, []string{"lvl"})
	tablesCompacted := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "tables_compacted",
	}, []string{"lvl"})
	tablesFlushed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "tables_flushed",
	}, []string{"lvl"})
	tablesIngested := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "tables_ingested",
	}, []string{"lvl"})
	tablesMoved := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "tables_moved",
	}, []string{"lvl"})
	for i := 0; i < dbLvlsAmount; i++ {
		label := strconv.Itoa(i)
		listener.numFiles[i] = numFiles.WithLabelValues(label)
		listener.size[i] = size.WithLabelValues(label)
		listener.score[i] = score.WithLabelValues(label)
		listener.bytesIn[i] = bytesIn.WithLabelValues(label)
		listener.bytesIngested[i] = bytesIngested.WithLabelValues(label)
		listener.bytesMoved[i] = bytesMoved.WithLabelValues(label)
		listener.bytesRead[i] = bytesRead.WithLabelValues(label)
		listener.bytesCompacted[i] = bytesCompacted.WithLabelValues(label)
		listener.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		listener.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		listener.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		listener.bytesFlushed[i] = bytesFlushed.WithLabelValues(label)
		listener.tablesCompacted[i] = tablesCompacted.WithLabelValues(label)
		listener.tablesFlushed[i] = tablesFlushed.WithLabelValues(label)
		listener.tablesIngested[i] = tablesIngested.WithLabelValues(label)
		listener.tablesMoved[i] = tablesMoved.WithLabelValues(label)
	}
	prometheus.MustRegister(
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
	return listener
}

// format formats provided levels metrics data into a collectable structure.
func (listener *levelsListener) format(levels *[7]pebble.LevelMetrics) {
	// swap cache
	cache := listener.cache
	listener.cache.Lvls = *levels
	for i := 0; i < len(levels); i++ {
		// These metrics are only ever increasing, use delta instead of total.
		levels[i].BytesIn -= cache.Lvls[i].BytesIn
		levels[i].BytesIngested -= cache.Lvls[i].BytesIngested
		levels[i].BytesMoved -= cache.Lvls[i].BytesMoved
		levels[i].BytesRead -= cache.Lvls[i].BytesRead
		levels[i].BytesCompacted -= cache.Lvls[i].BytesCompacted
		levels[i].BytesFlushed -= cache.Lvls[i].BytesFlushed
		levels[i].TablesCompacted -= cache.Lvls[i].TablesCompacted
		levels[i].TablesFlushed -= cache.Lvls[i].TablesFlushed
		levels[i].TablesIngested -= cache.Lvls[i].TablesIngested
		levels[i].TablesMoved -= cache.Lvls[i].TablesMoved
	}
}

// gather collects and updates level-specific metrics from pebble.
func (listener *levelsListener) gather(metrics *db.PebbleMetrics) {
	levels := metrics.Src.Levels
	listener.format(&levels)

	for i, lvl := range levels {
		listener.numFiles[i].Set(float64(lvl.NumFiles))
		listener.size[i].Set(float64(lvl.Size))
		listener.score[i].Set(lvl.Score)
		listener.bytesIn[i].Add(float64(lvl.BytesIn))
		listener.bytesIngested[i].Add(float64(lvl.BytesIngested))
		listener.bytesMoved[i].Add(float64(lvl.BytesMoved))
		listener.bytesRead[i].Add(float64(lvl.BytesRead))
		listener.bytesCompacted[i].Add(float64(lvl.BytesCompacted))
		listener.bytesFlushed[i].Add(float64(lvl.BytesFlushed))
		listener.tablesCompacted[i].Add(float64(lvl.TablesCompacted))
		listener.tablesFlushed[i].Add(float64(lvl.TablesFlushed))
		listener.tablesIngested[i].Add(float64(lvl.TablesIngested))
		listener.tablesMoved[i].Add(float64(lvl.TablesMoved))
	}
}

// compactionListener listens for pebble compaction metrics.
type compactionListener struct {
	// cache stores previous data in order to calculate delta.
	cache struct {
		DefaultCount     int64
		DeleteOnlyCount  int64
		ElisionOnlyCount int64
		MoveCount        int64
		ReadCount        int64
		RewriteCount     int64
		MultiLevelCount  int64
	}

	// metrics
	Defaults        prometheus.Counter
	DeletesOnly     prometheus.Counter
	ElisionsOnly    prometheus.Counter
	Moves           prometheus.Counter
	Reads           prometheus.Counter
	Rewrites        prometheus.Counter
	MultiLevels     prometheus.Counter
	EstimatedDebt   prometheus.Gauge
	InProgressBytes prometheus.Gauge
	NumInProgress   prometheus.Gauge
	MarkedFiles     prometheus.Gauge
}

// newCompactionListener creates and returns a new compactionListener instance with setup prometheus metrics.
func newCompactionListener() *compactionListener {
	const subsystem = "compaction"
	listener := &compactionListener{}
	listener.EstimatedDebt = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "estimated_debt",
	})
	listener.InProgressBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "in_progress_bytes",
	})
	listener.NumInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "num_in_progress",
	})
	listener.MarkedFiles = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "marked_files",
	})
	compactionsCounts := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "amount",
	}, []string{"type"})
	listener.Defaults = compactionsCounts.WithLabelValues("default")
	listener.DeletesOnly = compactionsCounts.WithLabelValues("delete")
	listener.ElisionsOnly = compactionsCounts.WithLabelValues("elision")
	listener.Moves = compactionsCounts.WithLabelValues("move")
	listener.Reads = compactionsCounts.WithLabelValues("read")
	listener.Rewrites = compactionsCounts.WithLabelValues("rewrite")
	listener.MultiLevels = compactionsCounts.WithLabelValues("multi_level")
	prometheus.MustRegister(
		compactionsCounts,
		listener.EstimatedDebt,
		listener.InProgressBytes,
		listener.NumInProgress,
		listener.MarkedFiles,
	)
	return listener
}

// updateCache updates the cache with new data and returns the older version.
func (listener *compactionListener) updateCache(stats *pebble.Metrics) struct {
	DefaultCount     int64
	DeleteOnlyCount  int64
	ElisionOnlyCount int64
	MoveCount        int64
	ReadCount        int64
	RewriteCount     int64
	MultiLevelCount  int64
} {
	cache := listener.cache
	listener.cache.DefaultCount = stats.Compact.DefaultCount
	listener.cache.DeleteOnlyCount = stats.Compact.DeleteOnlyCount
	listener.cache.ElisionOnlyCount = stats.Compact.ElisionOnlyCount
	listener.cache.MoveCount = stats.Compact.MoveCount
	listener.cache.ReadCount = stats.Compact.ReadCount
	listener.cache.RewriteCount = stats.Compact.RewriteCount
	listener.cache.MultiLevelCount = stats.Compact.MultiLevelCount
	return cache
}

// format formats provided data into collectable metrics.
func (listener *compactionListener) format(stats *pebble.Metrics) struct {
	DefaultCount     int64
	DeleteOnlyCount  int64
	ElisionOnlyCount int64
	MoveCount        int64
	ReadCount        int64
	RewriteCount     int64
	MultiLevelCount  int64
	EstimatedDebt    uint64
	InProgressBytes  int64
	NumInProgress    int64
	MarkedFiles      int
} {
	cache := listener.updateCache(stats)
	return struct {
		DefaultCount     int64
		DeleteOnlyCount  int64
		ElisionOnlyCount int64
		MoveCount        int64
		ReadCount        int64
		RewriteCount     int64
		MultiLevelCount  int64
		EstimatedDebt    uint64
		InProgressBytes  int64
		NumInProgress    int64
		MarkedFiles      int
	}{
		EstimatedDebt:   stats.Compact.EstimatedDebt,
		InProgressBytes: stats.Compact.InProgressBytes,
		NumInProgress:   stats.Compact.NumInProgress,
		MarkedFiles:     stats.Compact.MarkedFiles,
		// These metrics are only ever increasing, return delta instead of total.
		DefaultCount:     listener.cache.DefaultCount - cache.DefaultCount,
		DeleteOnlyCount:  listener.cache.DeleteOnlyCount - cache.DeleteOnlyCount,
		ElisionOnlyCount: listener.cache.ElisionOnlyCount - cache.ElisionOnlyCount,
		MoveCount:        listener.cache.MoveCount - cache.MoveCount,
		ReadCount:        listener.cache.ReadCount - cache.ReadCount,
		RewriteCount:     listener.cache.RewriteCount - cache.RewriteCount,
		MultiLevelCount:  listener.cache.MultiLevelCount - cache.MultiLevelCount,
	}
}

// gather collects and updates compaction-specific metrics from pebble.
func (listener *compactionListener) gather(metrics *db.PebbleMetrics) {
	formatted := listener.format(metrics.Src)

	listener.Defaults.Add(float64(formatted.DefaultCount))
	listener.DeletesOnly.Add(float64(formatted.DeleteOnlyCount))
	listener.ElisionsOnly.Add(float64(formatted.ElisionOnlyCount))
	listener.Moves.Add(float64(formatted.MoveCount))
	listener.Reads.Add(float64(formatted.ReadCount))
	listener.Rewrites.Add(float64(formatted.RewriteCount))
	listener.MultiLevels.Add(float64(formatted.MultiLevelCount))
	listener.EstimatedDebt.Set(float64(formatted.EstimatedDebt))
	listener.InProgressBytes.Set(float64(formatted.InProgressBytes))
	listener.NumInProgress.Set(float64(formatted.NumInProgress))
	listener.MarkedFiles.Set(float64(formatted.MarkedFiles))
}

// cacheListener listens for pebble cache metrics.
type cacheListener struct {
	// cache stores previous data in order to calculate delta.
	cache struct {
		Hits   uint64
		Misses uint64
	}

	Size   prometheus.Gauge
	Count  prometheus.Gauge
	Hits   prometheus.Counter
	Misses prometheus.Counter
}

// newCacheListener creates and returns a new cacheListener instance with setup prometheus metrics.
func newCacheListener() *cacheListener {
	const subsystem = "cache"
	listener := &cacheListener{}
	listener.Size = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "size",
	})
	listener.Count = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "count",
	})
	hitCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "hits",
	}, []string{"succesfull"})
	listener.Hits = hitCounter.WithLabelValues("true")
	listener.Misses = hitCounter.WithLabelValues("false")
	prometheus.MustRegister(
		listener.Size,
		listener.Count,
		hitCounter,
	)
	return listener
}

// updateCache updates the cache with new data and returns the older version.
func (listener *cacheListener) updateCache(stats pebble.CacheMetrics) struct {
	Hits   uint64
	Misses uint64
} {
	cache := listener.cache
	listener.cache.Hits = uint64(stats.Hits)
	listener.cache.Misses = uint64(stats.Misses)
	return cache
}

// gather collects and updates cache-specific metrics from pebble.
func (listener *cacheListener) gather(stats *db.PebbleMetrics) {
	cache := listener.updateCache(stats.Src.BlockCache)

	listener.Size.Set(float64(stats.Src.BlockCache.Size))
	listener.Count.Set(float64(stats.Src.BlockCache.Count))
	listener.Hits.Add(float64(listener.cache.Hits - cache.Hits))
	listener.Misses.Add(float64(listener.cache.Misses - cache.Misses))
}

// flushListener listens for pebble flush metrics.
type flushListener struct {
	// cache stores previous data in order to calculate delta.
	cache struct {
		AsIngestCount      uint64
		AsIngestTableCount uint64
		AsIngestBytes      uint64
		BytesProcessed     uint64
		Count              uint64
		WorkDuration       time.Duration
		IdleDuration       time.Duration
	}

	Count              prometheus.Counter
	AsIngestCount      prometheus.Counter
	AsIngestTableCount prometheus.Counter
	AsIngestBytes      prometheus.Counter
	BytesProcessed     prometheus.Counter
	NumInProgress      prometheus.Gauge
	WorkDuration       prometheus.Counter
	IdleDuration       prometheus.Counter
}

// newFlushListener creates and returns a new flushListener instance with setup prometheus metrics.
func newFlushListener() *flushListener {
	const subsystem = "flush"
	listener := &flushListener{}
	listener.Count = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "amount",
	})
	listener.AsIngestCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "ingests",
	})
	listener.AsIngestTableCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "ingest_tables",
	})
	listener.AsIngestBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "ingest_bytes",
	})
	listener.BytesProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "bytes_processed",
	})
	listener.NumInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "in_progress",
	})
	workCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "work",
	}, []string{"state"})
	listener.WorkDuration = workCounter.WithLabelValues("work")
	listener.IdleDuration = workCounter.WithLabelValues("idle")
	prometheus.MustRegister(
		listener.Count,
		listener.AsIngestCount,
		listener.AsIngestTableCount,
		listener.AsIngestBytes,
		listener.NumInProgress,
		listener.BytesProcessed,
		workCounter,
	)
	return listener
}

// updateCache updates the cache with new data and returns the older version.
func (listener *flushListener) updateCache(stats *pebble.Metrics) struct {
	AsIngestCount      uint64
	AsIngestTableCount uint64
	AsIngestBytes      uint64
	BytesProcessed     uint64
	Count              uint64
	WorkDuration       time.Duration
	IdleDuration       time.Duration
} {
	cache := listener.cache
	listener.cache.Count = uint64(stats.Flush.Count)
	listener.cache.AsIngestCount = stats.Flush.AsIngestCount
	listener.cache.AsIngestTableCount = stats.Flush.AsIngestTableCount
	listener.cache.AsIngestBytes = stats.Flush.AsIngestBytes
	listener.cache.BytesProcessed = uint64(stats.Flush.WriteThroughput.Bytes)
	listener.cache.IdleDuration = stats.Flush.WriteThroughput.IdleDuration
	listener.cache.WorkDuration = stats.Flush.WriteThroughput.WorkDuration
	return cache
}

// format formats provided data into collectable metrics.
func (listener *flushListener) format(stats *pebble.Metrics) struct {
	Count              uint64
	AsIngestCount      uint64
	AsIngestTableCount uint64
	AsIngestBytes      uint64
	BytesProcessed     uint64
	IdleDuration       time.Duration
	WorkDuration       time.Duration
} {
	cache := listener.updateCache(stats)
	return struct {
		Count              uint64
		AsIngestCount      uint64
		AsIngestTableCount uint64
		AsIngestBytes      uint64
		BytesProcessed     uint64
		IdleDuration       time.Duration
		WorkDuration       time.Duration
	}{
		Count:              listener.cache.Count - cache.Count,
		AsIngestCount:      listener.cache.AsIngestCount - cache.AsIngestCount,
		AsIngestTableCount: listener.cache.AsIngestTableCount - cache.AsIngestTableCount,
		AsIngestBytes:      listener.cache.AsIngestBytes - cache.AsIngestBytes,
		BytesProcessed:     listener.cache.BytesProcessed - cache.BytesProcessed,
		IdleDuration:       listener.cache.IdleDuration - cache.IdleDuration,
		WorkDuration:       listener.cache.WorkDuration - cache.WorkDuration,
	}
}

// gather collects and updates flush-specific metrics from pebble.
func (listener *flushListener) gather(stats *db.PebbleMetrics) {
	formatted := listener.format(stats.Src)

	listener.Count.Add(float64(formatted.Count))
	listener.AsIngestCount.Add(float64(formatted.AsIngestCount))
	listener.AsIngestTableCount.Add(float64(formatted.AsIngestTableCount))
	listener.AsIngestBytes.Add(float64(formatted.AsIngestBytes))
	listener.BytesProcessed.Add(float64(formatted.BytesProcessed))
	listener.IdleDuration.Add((formatted.IdleDuration).Seconds())
	listener.WorkDuration.Add((formatted.WorkDuration).Seconds())
}

// flushListener listens for pebble filter metrics.
type filterListener struct {
	// cache stores previous data in order to calculate delta.
	cache struct {
		Hits   uint64
		Misses uint64
	}

	Hits   prometheus.Counter
	Misses prometheus.Counter
}

// newFilterListener creates and returns a new filterListener instance with setup prometheus metrics.
func newFilterListener() *filterListener {
	const subsystem = "filter"
	listener := &filterListener{}
	filterCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "hits",
	}, []string{"succesfull"})
	listener.Hits = filterCounter.WithLabelValues("true")
	listener.Misses = filterCounter.WithLabelValues("false")
	prometheus.MustRegister(filterCounter)
	return listener
}

// updateCache updates the cache with new data and returns the older version.
func (listener *filterListener) updateCache(stats pebble.FilterMetrics) struct {
	Hits   uint64
	Misses uint64
} {
	cache := listener.cache
	listener.cache.Hits = uint64(stats.Hits)
	listener.cache.Misses = uint64(stats.Misses)
	return cache
}

// format formats provided data into collectable metrics.
func (listener *filterListener) format(stats pebble.FilterMetrics) struct {
	Hits   uint64
	Misses uint64
} {
	cache := listener.updateCache(stats)
	return struct {
		Hits   uint64
		Misses uint64
	}{
		Hits:   listener.cache.Hits - cache.Hits,
		Misses: listener.cache.Misses - cache.Misses,
	}
}

// gather collects and updates filter-specific metrics from pebble.
func (listener *filterListener) gather(stats *db.PebbleMetrics) {
	formatted := listener.format(stats.Src.Filter)

	listener.Hits.Add(float64(formatted.Hits))
	listener.Misses.Add(float64(formatted.Misses))
}

// memtableListener listens for pebble memtable metrics.
type memtableListener struct {
	Size       prometheus.Gauge
	Count      prometheus.Gauge
	ZombieSize prometheus.Gauge
	Zombies    prometheus.Gauge
}

// newMemtableListener creates and returns a new memtableListener instance with setup prometheus metrics.
func newMemtableListener() *memtableListener {
	const subsystem = "memtable"
	listener := &memtableListener{}
	listener.Size = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "size",
	})
	listener.Count = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "amount",
	})
	listener.ZombieSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "zombie_size",
	})
	listener.Zombies = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "zombies",
	})
	prometheus.MustRegister(
		listener.Size,
		listener.Count,
		listener.ZombieSize,
		listener.Zombies,
	)
	return listener
}

// gather collects and updates memtable-specific metrics from pebble.
func (listener *memtableListener) gather(stats *db.PebbleMetrics) {
	listener.Size.Set(float64(stats.Src.MemTable.Size))
	listener.Count.Set(float64(stats.Src.MemTable.Count))
	listener.ZombieSize.Set(float64(stats.Src.MemTable.ZombieSize))
	listener.Zombies.Set(float64(stats.Src.MemTable.ZombieCount))
}

// keysListener listens for pebble keys metrics.
type keysListener struct {
	RangeKeySets prometheus.Gauge
	Tombstones   prometheus.Gauge
}

// newKeysListener creates and returns a new keysListener instance with setup prometheus metrics.
func newKeysListener() *keysListener {
	const subsystem = "keys"
	listener := &keysListener{}
	listener.RangeKeySets = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "range_key_sets",
	})
	listener.Tombstones = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "tombstones",
	})
	prometheus.MustRegister(
		listener.RangeKeySets,
		listener.Tombstones,
	)
	return listener
}

// gather collects and updates keys-specific metrics from pebble.
func (listener *keysListener) gather(stats *db.PebbleMetrics) {
	listener.RangeKeySets.Set(float64(stats.Src.Keys.RangeKeySetsCount))
	listener.Tombstones.Set(float64(stats.Src.Keys.TombstoneCount))
}

// snapshotsListener listens for pebble snapshot metrics.
type snapshotsListener struct {
	// cache stores previous data in order to calculate delta.
	cache struct {
		PinnedKeys uint64
		PinnedSize uint64
	}

	Count          prometheus.Gauge
	EarliestSeqNum prometheus.Gauge
	PinnedKeys     prometheus.Counter
	PinnedSize     prometheus.Counter
}

// newSnapshotListener creates and returns a new snapshotsListener instance with setup prometheus metrics.
func newSnapshotListener() *snapshotsListener {
	const subsystem = "snapshots"
	listener := &snapshotsListener{}
	listener.Count = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "amount",
	})
	listener.EarliestSeqNum = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "earliest_seq_num",
	})
	listener.PinnedKeys = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "pinned_keys",
	})
	listener.PinnedSize = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Subsystem: subsystem,
		Name:      "pinned_size",
	})
	prometheus.MustRegister(
		listener.Count,
		listener.EarliestSeqNum,
		listener.PinnedKeys,
		listener.PinnedSize,
	)
	return listener
}

// updateCache updates the cache with new data and returns the older version.
func (listener *snapshotsListener) updateCache(stats *pebble.Metrics) struct {
	PinnedKeys uint64
	PinnedSize uint64
} {
	cache := listener.cache
	listener.cache.PinnedKeys = stats.Snapshots.PinnedKeys
	listener.cache.PinnedSize = stats.Snapshots.PinnedSize
	return cache
}

// format formats provided data into collectable metrics.
func (listener *snapshotsListener) format(stats *pebble.Metrics) struct {
	Count          int
	EarliestSeqNum uint64
	PinnedKeys     uint64
	PinnedSize     uint64
} {
	cache := listener.updateCache(stats)
	return struct {
		Count          int
		EarliestSeqNum uint64
		PinnedKeys     uint64
		PinnedSize     uint64
	}{
		Count:          stats.Snapshots.Count,
		EarliestSeqNum: stats.Snapshots.EarliestSeqNum,
		PinnedKeys:     listener.cache.PinnedKeys - cache.PinnedKeys,
		PinnedSize:     listener.cache.PinnedSize - cache.PinnedSize,
	}
}

// gather collects and updates snapshot-specific metrics from pebble.
func (listener *snapshotsListener) gather(stats *db.PebbleMetrics) {
	formatted := listener.format(stats.Src)

	listener.Count.Set(float64(formatted.Count))
	listener.EarliestSeqNum.Set(float64(formatted.EarliestSeqNum))
	listener.PinnedKeys.Add(float64(formatted.PinnedKeys))
	listener.PinnedSize.Add(float64(formatted.PinnedSize))
}

func makeDBMetrics() db.EventListener {
	readCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "db",
		Name:      "read",
	})
	writeCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "db",
		Name:      "write",
	})
	prometheus.MustRegister(readCounter, writeCounter)
	pebbleListener := newPebbleListener()
	return &db.SelectiveListener{
		OnIOCb: func(write bool) {
			if write {
				writeCounter.Inc()
			} else {
				readCounter.Inc()
			}
		},
		OnPebbleMetricsCb: pebbleListener.gather,
	}
}

func makeHTTPMetrics() jsonrpc.NewRequestListener {
	reqCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "http",
		Name:      "requests",
	})
	prometheus.MustRegister(reqCounter)

	return &jsonrpc.SelectiveListener{
		OnNewRequestCb: func(method string) {
			reqCounter.Inc()
		},
	}
}

func makeWSMetrics() jsonrpc.NewRequestListener {
	reqCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "ws",
		Name:      "requests",
	})
	prometheus.MustRegister(reqCounter)

	return &jsonrpc.SelectiveListener{
		OnNewRequestCb: func(method string) {
			reqCounter.Inc()
		},
	}
}

func makeRPCMetrics(version, legacyVersion string) (jsonrpc.EventListener, jsonrpc.EventListener) {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "server",
		Name:      "requests",
	}, []string{"method", "version"})
	failedRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "rpc",
		Subsystem: "server",
		Name:      "failed_requests",
	}, []string{"method", "version"})
	requestLatencies := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "rpc",
		Subsystem: "server",
		Name:      "requests_latency",
	}, []string{"method", "version"})
	prometheus.MustRegister(requests, failedRequests, requestLatencies)

	return &jsonrpc.SelectiveListener{
			OnNewRequestCb: func(method string) {
				requests.WithLabelValues(method, version).Inc()
			},
			OnRequestHandledCb: func(method string, took time.Duration) {
				requestLatencies.WithLabelValues(method, version).Observe(took.Seconds())
			},
			OnRequestFailedCb: func(method string, data any) {
				failedRequests.WithLabelValues(method, version).Inc()
			},
		}, &jsonrpc.SelectiveListener{
			OnNewRequestCb: func(method string) {
				requests.WithLabelValues(method, legacyVersion).Inc()
			},
			OnRequestHandledCb: func(method string, took time.Duration) {
				requestLatencies.WithLabelValues(method, legacyVersion).Observe(took.Seconds())
			},
			OnRequestFailedCb: func(method string, data any) {
				failedRequests.WithLabelValues(method, legacyVersion).Inc()
			},
		}
}

func makeSyncMetrics(syncReader sync.Reader, bcReader blockchain.Reader) sync.EventListener {
	opTimerHistogram := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "sync",
		Name:      "timers",
	}, []string{"op"})
	blockCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "sync",
		Name:      "blocks",
	})
	reorgCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "sync",
		Name:      "reorganisations",
	})
	chainHeightGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sync",
		Name:      "blockchain_height",
	}, func() float64 {
		height, _ := bcReader.Height()
		return float64(height)
	})
	bestBlockGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "sync",
		Name:      "best_known_block_number",
	}, func() float64 {
		bestHeader := syncReader.HighestBlockHeader()
		if bestHeader != nil {
			return float64(bestHeader.Number)
		}
		return 0
	})

	prometheus.MustRegister(opTimerHistogram, blockCount, chainHeightGauge, bestBlockGauge, reorgCount)

	return &sync.SelectiveListener{
		OnSyncStepDoneCb: func(op string, blockNum uint64, took time.Duration) {
			opTimerHistogram.WithLabelValues(op).Observe(took.Seconds())
			if op == sync.OpStore {
				blockCount.Inc()
			}
		},
		OnReorgCb: func(blockNum uint64) {
			reorgCount.Inc()
		},
	}
}
