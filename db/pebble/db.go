package pebble

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// minCache is the minimum amount of memory in megabytes to allocate to pebble read and write caching.
	// This is also pebble's default value.
	minCacheSizeMB = 8

	// metricsGatheringInterval specifies the interval to retrieve pebble database
	// compaction, io and pause stats to report to the user.
	metricsGatheringInterval = 3 * time.Second

	// dbNamespace is the namespace for the database metrics
	dbNamespace = "db"
)

var (
	ErrDiscardedTransaction = errors.New("discarded transaction")
	ErrReadOnlyTransaction  = errors.New("read-only transaction")
)
var _ db.DB = (*DB)(nil)

type DB struct {
	pebble   *pebble.DB
	wMutex   *sync.Mutex
	listener db.EventListener

	compTimeMeter        prometheus.Counter // Total time spent in database compaction
	compReadMeter        prometheus.Counter // Total bytes read during compaction
	compWriteMeter       prometheus.Counter // Total bytes written during compaction
	writeDelayCountMeter prometheus.Counter // Total write delay counts due to database compaction
	writeDelayTimeMeter  prometheus.Counter // Total write delay duration due to database compaction
	diskSizeGauge        prometheus.Gauge   // Tracks the size of all of the levels of the database
	diskWriteMeter       prometheus.Counter // Measures the effective amount of data written to disk
	memCompGauge         prometheus.Gauge   // Tracks the amount of memory allocated for compaction
	level0CompGauge      prometheus.Gauge   // Tracks the number of level-zero compactions
	nonlevel0CompGauge   prometheus.Gauge   // Tracks the number of non level-zero compactions
	seekCompGauge        prometheus.Gauge   // Tracks the number of table compaction caused by read opt
	manualMemAllocGauge  prometheus.Gauge   // Tracks the amount of non-managed memory currently allocated

	activeComp          int           // Current number of active compactions
	compStartTime       time.Time     // The start time of the earliest currently-active compaction
	compTime            atomic.Int64  // Total time spent in compaction in ns
	level0Comp          atomic.Uint32 // Total number of level-zero compactions
	nonLevel0Comp       atomic.Uint32 // Total number of non level-zero compactions
	writeDelayStartTime time.Time     // The start time of the latest write stall
	writeDelayCount     atomic.Int64  // Total number of write stall counts
	writeDelayTime      atomic.Int64  // Total time spent in write stalls
}

func New(path string, enableMetrics bool, options ...Option) (*DB, error) {
	opts := &pebble.Options{
		MaxConcurrentCompactions: func() int { return runtime.NumCPU() },
	}

	for _, option := range options {
		option(opts)
	}

	return newPebble(path, enableMetrics, opts)
}

// NewMem opens a new in-memory database
func NewMem() (*DB, error) {
	return newPebble("", false, &pebble.Options{
		FS: vfs.NewMem(),
	})
}

// NewMemTest opens a new in-memory database, panics on error
func NewMemTest(t *testing.T) *DB {
	memDB, err := NewMem()
	if err != nil {
		t.Fatalf("create in-memory db: %v", err)
	}
	t.Cleanup(func() {
		if err := memDB.Close(); err != nil {
			t.Errorf("close in-memory db: %v", err)
		}
	})
	return memDB
}

func newPebble(path string, enableMetrics bool, options *pebble.Options) (*DB, error) {
	database := &DB{
		wMutex:   new(sync.Mutex),
		listener: &db.SelectiveListener{},
	}

	if enableMetrics {
		database.enableMetrics()

		options.EventListener = &pebble.EventListener{
			CompactionBegin: database.onCompactionBegin,
			CompactionEnd:   database.onCompactionEnd,
			WriteStallBegin: database.onWriteStallBegin,
			WriteStallEnd:   database.onWriteStallEnd,
		}
	}

	pDB, err := pebble.Open(path, options)
	if err != nil {
		return nil, err
	}
	database.pebble = pDB

	return database, nil
}

// Option is a functional option for configuring the DB
type Option func(*pebble.Options)

// WithCacheSize sets the cache size in megabytes
func WithCacheSize(cacheSizeMB uint) Option {
	return func(opts *pebble.Options) {
		// Ensure that the specified cache size meets a minimum threshold.
		cacheSizeMB = max(cacheSizeMB, minCacheSizeMB)
		opts.Cache = pebble.NewCache(int64(cacheSizeMB * utils.Megabyte))
	}
}

// WithMaxOpenFiles sets the maximum number of open files
func WithMaxOpenFiles(maxOpenFiles int) Option {
	return func(opts *pebble.Options) {
		opts.MaxOpenFiles = maxOpenFiles
	}
}

// WithColouredLogger sets whether to use a coloured logger
func WithColouredLogger(coloured bool) Option {
	return func(opts *pebble.Options) {
		dbLog, err := utils.NewZapLogger(utils.ERROR, coloured)
		if err != nil {
			// Since we can't return an error from this function, we'll panic instead
			panic(fmt.Errorf("create DB logger: %w", err))
		}
		opts.Logger = dbLog
	}
}

// WithListener registers an EventListener
func (d *DB) WithListener(listener db.EventListener) db.DB {
	d.listener = listener
	return d
}

// EnableMetrics enables metrics collection for the database
func (d *DB) enableMetrics() db.DB {
	d.compTimeMeter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Name:      "compaction_time",
		Help:      "Total time spent in database compaction",
	})
	d.compReadMeter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Name:      "compaction_read",
		Help:      "Total bytes read during compaction",
	})
	d.compWriteMeter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Name:      "compaction_write",
		Help:      "Total bytes written during compaction",
	})
	d.writeDelayCountMeter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Name:      "write_delay_count",
		Help:      "Total write delay counts due to database compaction",
	})
	d.writeDelayTimeMeter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Name:      "write_delay_time",
		Help:      "Total time spent in write stalls",
	})
	d.diskSizeGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Name:      "disk_size",
		Help:      "Tracks the size of all of the levels of the database",
	})
	d.diskWriteMeter = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: dbNamespace,
		Name:      "disk_write",
		Help:      "Measures the effective amount of data written to disk",
	})
	d.memCompGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Name:      "mem_comp",
		Help:      "Tracks the amount of memory allocated for compaction",
	})
	d.level0CompGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Name:      "level0_comp",
		Help:      "Tracks the number of level-zero compactions",
	})
	d.nonlevel0CompGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Name:      "nonlevel0_comp",
		Help:      "Tracks the number of non level-zero compactions",
	})
	d.seekCompGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Name:      "seek_comp",
		Help:      "Tracks the number of table compaction caused by read opt",
	})
	d.manualMemAllocGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: dbNamespace,
		Name:      "manual_mem_alloc",
		Help:      "Tracks the amount of non-managed memory currently allocated",
	})

	prometheus.MustRegister(
		d.compTimeMeter,
		d.compReadMeter,
		d.compWriteMeter,
		d.writeDelayCountMeter,
		d.writeDelayTimeMeter,
		d.diskSizeGauge,
		d.diskWriteMeter,
		d.memCompGauge,
		d.level0CompGauge,
		d.nonlevel0CompGauge,
		d.seekCompGauge,
		d.manualMemAllocGauge,
	)

	return d
}

func (d *DB) onCompactionBegin(info pebble.CompactionInfo) {
	if d.activeComp == 0 {
		d.compStartTime = time.Now()
	}
	l0 := info.Input[0]
	if l0.Level == 0 {
		d.level0Comp.Add(1)
	} else {
		d.nonLevel0Comp.Add(1)
	}
	d.activeComp++
}

func (d *DB) onCompactionEnd(info pebble.CompactionInfo) {
	if d.activeComp == 1 {
		d.compTime.Add(int64(time.Since(d.compStartTime)))
	} else if d.activeComp == 0 {
		panic("should not happen")
	}
	d.activeComp--
}

func (d *DB) onWriteStallBegin(b pebble.WriteStallBeginInfo) {
	d.writeDelayStartTime = time.Now()
}

func (d *DB) onWriteStallEnd() {
	d.writeDelayTime.Add(int64(time.Since(d.writeDelayStartTime)))
}

// collectMetrics periodically retrieves internal pebble counters and reports them to
// the metrics subsystem.
func (d *DB) StartMetricsCollection(ctx context.Context, refresh time.Duration) {
	timer := time.NewTimer(refresh)
	defer timer.Stop()

	// Create storage and warning log tracer for write delay.
	var (
		compTimes        [2]int64
		writeDelayTimes  [2]int64
		writeDelayCounts [2]int64
		compWrites       [2]int64
		compReads        [2]int64

		nWrites [2]int64
	)

	// Iterate ad infinitum and collect the stats
	for i := 1; ; i++ {
		var (
			compWrite int64
			compRead  int64
			nWrite    int64

			metrics            = d.pebble.Metrics()
			compTime           = d.compTime.Load()
			writeDelayCount    = d.writeDelayCount.Load()
			writeDelayTime     = d.writeDelayTime.Load()
			nonLevel0CompCount = int64(d.nonLevel0Comp.Load())
			level0CompCount    = int64(d.level0Comp.Load())
		)
		writeDelayTimes[i%2] = writeDelayTime
		writeDelayCounts[i%2] = writeDelayCount
		compTimes[i%2] = compTime

		for _, levelMetrics := range metrics.Levels {
			nWrite += int64(levelMetrics.BytesCompacted)
			nWrite += int64(levelMetrics.BytesFlushed)
			compWrite += int64(levelMetrics.BytesCompacted)
			compRead += int64(levelMetrics.BytesRead)
		}

		nWrite += int64(metrics.WAL.BytesWritten)

		compWrites[i%2] = compWrite
		compReads[i%2] = compRead
		nWrites[i%2] = nWrite

		d.writeDelayCountMeter.Add(float64(writeDelayCounts[i%2] - writeDelayCounts[(i-1)%2]))
		d.writeDelayTimeMeter.Add(float64(writeDelayTimes[i%2] - writeDelayTimes[(i-1)%2]))
		d.compTimeMeter.Add(float64(compTimes[i%2] - compTimes[(i-1)%2]))
		d.compReadMeter.Add(float64(compReads[i%2] - compReads[(i-1)%2]))
		d.compWriteMeter.Add(float64(compWrites[i%2] - compWrites[(i-1)%2]))
		d.diskSizeGauge.Set(float64(metrics.DiskSpaceUsage()))
		d.diskWriteMeter.Add(float64(nWrites[i%2] - nWrites[(i-1)%2]))

		// See https://github.com/cockroachdb/pebble/pull/1628#pullrequestreview-1026664054
		manuallyAllocated := metrics.BlockCache.Size + int64(metrics.MemTable.Size) + int64(metrics.MemTable.ZombieSize)
		d.manualMemAllocGauge.Set(float64(manuallyAllocated))
		d.memCompGauge.Set(float64(metrics.Flush.Count))
		d.nonlevel0CompGauge.Set(float64(nonLevel0CompCount))
		d.level0CompGauge.Set(float64(level0CompCount))
		d.seekCompGauge.Set(float64(metrics.Compact.ReadCount))

		// Refresh the timer
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			timer.Reset(refresh)
		}
	}
}

// NewTransaction : see db.DB.NewTransaction
// Batch is used for read-write operations, while snapshot is used for read-only operations
func (d *DB) NewTransaction(update bool) (db.Transaction, error) {
	if update {
		d.wMutex.Lock()
		return NewBatch(d.pebble.NewIndexedBatch(), d.wMutex, d.listener), nil
	}

	return NewSnapshot(d.pebble.NewSnapshot(), d.listener), nil
}

// Close : see io.Closer.Close
func (d *DB) Close() error {
	return d.pebble.Close()
}

// View : see db.DB.View
func (d *DB) View(fn func(txn db.Transaction) error) error {
	return db.View(d, fn)
}

// Update : see db.DB.Update
func (d *DB) Update(fn func(txn db.Transaction) error) error {
	return db.Update(d, fn)
}

// Impl : see db.DB.Impl
func (d *DB) Impl() any {
	return d.pebble
}

type Item struct {
	Count uint
	Size  utils.DataSize
}

func (i *Item) add(size utils.DataSize) {
	i.Count++
	i.Size += size
}

func CalculatePrefixSize(ctx context.Context, pDB *DB, prefix []byte) (*Item, error) {
	var (
		err error
		v   []byte

		item = &Item{}
	)

	const upperBoundofPrefix = 0xff
	pebbleDB := pDB.Impl().(*pebble.DB)
	it, err := pebbleDB.NewIter(&pebble.IterOptions{LowerBound: prefix, UpperBound: append(prefix, upperBoundofPrefix)})
	if err != nil {
		// No need to call utils.RunAndWrapOnError() since iterator couldn't be created
		return nil, err
	}

	for it.First(); it.Valid(); it.Next() {
		if ctx.Err() != nil {
			return item, utils.RunAndWrapOnError(it.Close, ctx.Err())
		}
		v, err = it.ValueAndErr()
		if err != nil {
			return nil, utils.RunAndWrapOnError(it.Close, err)
		}

		item.add(utils.DataSize(len(it.Key()) + len(v)))
	}

	return item, utils.RunAndWrapOnError(it.Close, err)
}
