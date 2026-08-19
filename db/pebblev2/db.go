package pebblev2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/sstable"
	"golang.org/x/sync/errgroup"
)

var (
	ErrDiscardedTransaction = errors.New("discarded transaction")
	ErrReadOnlyTransaction  = errors.New("read-only transaction")
)

var _ db.KeyValueStore = (*DB)(nil)

type DB struct {
	db        *pebble.DB
	path      string
	closed    bool
	writeOpt  *pebble.WriteOptions
	listener  db.EventListener
	closeLock *sync.RWMutex // Ensures that the database is closed correctly
	// compactionConcurrency mirrors the options' upper compaction concurrency
	// bound, which pebble does not expose back; CompactAll fans out this many
	// concurrent manual compactions.
	compactionConcurrency int
	// tableFilterPolicy and tableCompression mirror the options' bottom-level
	// table settings; forced compaction skips tables whose properties already
	// match both, making it resumable. Empty when not configured, which
	// disables skipping.
	tableFilterPolicy string
	tableCompression  string
}

// New opens a new database at the given path with default options
func New(path string, options ...Option) (db.KeyValueStore, error) {
	version, err := upgradeFormatIfNeeded(path)
	if err != nil {
		return nil, err
	}

	opts := pebble.Options{
		FormatMajorVersion: version,
	}
	for _, option := range options {
		if err := option(&opts); err != nil {
			return nil, err
		}
	}

	bottomLevel := &opts.Levels[len(opts.Levels)-1]
	var tableFilterPolicy, tableCompression string
	if bottomLevel.FilterPolicy != nil {
		tableFilterPolicy = bottomLevel.FilterPolicy.Name()
	}
	if bottomLevel.Compression != nil {
		tableCompression = bottomLevel.Compression().Name
	}

	pDB, err := pebble.Open(path, &opts)
	if err != nil {
		return nil, err
	}

	compactionConcurrency := 1 // pebble's default range is (1, 1)
	if opts.CompactionConcurrencyRange != nil {
		_, upper := opts.CompactionConcurrencyRange()
		compactionConcurrency = max(upper, 1)
	}

	return &DB{
		db:                    pDB,
		path:                  path,
		closeLock:             new(sync.RWMutex),
		listener:              &db.SelectiveListener{},
		writeOpt:              &pebble.WriteOptions{Sync: true}, // TODO: can we use non-sync writes for performance?
		compactionConcurrency: compactionConcurrency,
		tableFilterPolicy:     tableFilterPolicy,
		tableCompression:      tableCompression,
	}, nil
}

func (d *DB) Path() string {
	return d.path
}

func (d *DB) Close() error {
	d.closeLock.Lock()
	defer d.closeLock.Unlock()

	if d.closed {
		return nil
	}
	d.closed = true

	return d.db.Close()
}

func (d *DB) Update(fn func(w db.IndexedBatch) error) error {
	if d.closed {
		return pebble.ErrClosed
	}

	batch := d.NewIndexedBatch()
	if err := fn(batch); err != nil {
		return err
	}

	return batch.Write()
}

func (d *DB) Write(fn func(w db.Batch) error) error {
	if d.closed {
		return pebble.ErrClosed
	}

	batch := d.NewBatch()
	if err := fn(batch); err != nil {
		return err
	}

	return batch.Write()
}

func (d *DB) WithListener(listener db.EventListener) db.KeyValueStore {
	d.listener = listener
	return d
}

func (d *DB) Impl() any {
	return d.db
}

func (d *DB) Has(key []byte) (bool, error) {
	defer d.listener.OnIO(false, time.Now())

	d.closeLock.RLock()
	defer d.closeLock.RUnlock()

	if d.closed {
		return false, pebble.ErrClosed
	}

	_, closer, err := d.db.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, errors.Join(err, closer.Close())
}

func (d *DB) Get(key []byte, cb func(value []byte) error) error {
	defer d.listener.OnIO(false, time.Now())

	d.closeLock.RLock()
	defer d.closeLock.RUnlock()

	if d.closed {
		return pebble.ErrClosed
	}

	val, closer, err := d.db.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return db.ErrKeyNotFound
		}
		return err
	}

	err = cb(val)
	return errors.Join(err, closer.Close())
}

func (d *DB) Put(key, val []byte) error {
	// Direct write to database also pays the cost of commit.
	defer d.listener.OnCommit(time.Now())

	d.closeLock.RLock()
	defer d.closeLock.RUnlock()

	if d.closed {
		return pebble.ErrClosed
	}

	return d.db.Set(key, val, d.writeOpt)
}

func (d *DB) Delete(key []byte) error {
	// Direct write to database also pays the cost of commit.
	defer d.listener.OnCommit(time.Now())

	d.closeLock.RLock()
	defer d.closeLock.RUnlock()

	if d.closed {
		return pebble.ErrClosed
	}

	return d.db.Delete(key, d.writeOpt)
}

func (d *DB) DeleteRange(start, end []byte) error {
	// Direct write to database also pays the cost of commit.
	defer d.listener.OnCommit(time.Now())

	d.closeLock.RLock()
	defer d.closeLock.RUnlock()

	if d.closed {
		return pebble.ErrClosed
	}

	return d.db.DeleteRange(start, end, d.writeOpt)
}

func (d *DB) NewBatch() db.Batch {
	return NewBatch(d.db.NewBatch(), d, d.listener)
}

func (d *DB) NewBatchWithSize(size int) db.Batch {
	return NewBatch(d.db.NewBatchWithSize(size), d, d.listener)
}

func (d *DB) NewIndexedBatch() db.IndexedBatch {
	return NewBatch(d.db.NewIndexedBatch(), d, d.listener)
}

func (d *DB) NewIndexedBatchWithSize(size int) db.IndexedBatch {
	return NewBatch(d.db.NewIndexedBatchWithSize(size), d, d.listener)
}

func (d *DB) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
	d.closeLock.RLock()
	defer d.closeLock.RUnlock()

	if d.closed {
		return nil, pebble.ErrClosed
	}

	iterOpt := &pebble.IterOptions{LowerBound: prefix}
	if withUpperBound {
		iterOpt.UpperBound = dbutils.UpperBound(prefix)
	}

	it, err := d.db.NewIter(iterOpt)
	if err != nil {
		return nil, err
	}

	return &iterator{iter: it, listener: d.listener}, nil
}

func (d *DB) NewSnapshot() db.Snapshot {
	return NewSnapshot(d.db, d.listener)
}

const (
	// maxForceCompactPasses bounds the forced-rewrite loop. A stale table
	// escaping a pass via a move compaction descends at least one level, so
	// passes beyond the level count mean no progress is possible.
	maxForceCompactPasses = 8

	// forceCompactChunkTables is the target number of stale tables per forced
	// compaction chunk. Chunks cover disjoint key ranges and compact
	// independently, letting pebble run them concurrently — a single
	// full-range manual compaction has one contiguous in-use range and runs
	// on one goroutine no matter the compaction concurrency.
	forceCompactChunkTables = 16
)

// forceChunk is one independently-compactable slice of the forced rewrite.
type forceChunk struct {
	start, end []byte
	// bottomSmallest holds the smallest key of every stale bottom-level table
	// in range; each gets a rewrite marker planted at it.
	bottomSmallest [][]byte
}

// CompactAll rewrites every sstable by compacting the full key range,
// materializing the current table options (bloom filters, compression)
// across existing data. Manual compaction rewrites bottom-level tables only
// as outputs of a compaction from the level above, so on an already-compacted
// database it picks nothing and rewrites nothing. force overwrites every
// stale bottom-level table's smallest key with its current value first,
// making each table an output of a real compaction, and repeats until no
// table predating the call remains: single-file compactions with no overlap
// below are "moves" that relink the old file a level down without rewriting
// it, so one pass is not always enough. Tables whose properties already match
// the configured filter policy and compression are skipped, so an interrupted
// forced compaction resumes instead of starting over. Force mode expects a
// database opened with WithOfflineCompaction — automatic compactions may
// otherwise merge the per-chunk marker sstables and serialize the chunk
// compactions.
func (d *DB) CompactAll(ctx context.Context, force bool) error {
	d.closeLock.RLock()
	defer d.closeLock.RUnlock()

	if d.closed {
		return pebble.ErrClosed
	}

	if !force {
		return d.db.Compact(ctx, nil, bytes.Repeat([]byte{0xff}, 8), true)
	}

	// Rewritten tables always receive a fresh file number, moved tables keep
	// theirs: tables numbered at or below the baseline still need a rewrite.
	baseline, err := d.maxTableNum()
	if err != nil {
		return err
	}

	for range maxForceCompactPasses {
		chunks, err := d.staleChunks(baseline)
		if err != nil {
			return err
		}
		if len(chunks) == 0 {
			return nil
		}

		if err := d.compactChunks(ctx, chunks); err != nil {
			return err
		}
	}

	stale, err := d.staleChunks(baseline)
	if err != nil {
		return err
	}
	if len(stale) == 0 {
		return nil
	}
	return fmt.Errorf("stale sstables remain after %d passes", maxForceCompactPasses)
}

// compactChunks plants each chunk's markers and compacts its range, fanning
// the chunks out so pebble can run their compactions concurrently. Planting
// and flushing are serialized so every chunk's markers land in own sstables
// bounded to its range; a carrier spanning several chunks would make their
// compactions conflict on it and serialize. Chunks launch in stride order —
// key-space neighbors can share a straddling upper-level table, and pebble
// stalls the whole manual-compaction queue while its head conflicts with a
// running compaction, so the concurrently-active set is kept spread out.
func (d *DB) compactChunks(ctx context.Context, chunks []forceChunk) error {
	var plantMu sync.Mutex
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(d.compactionConcurrency)

	stride := max(1, (len(chunks)+d.compactionConcurrency-1)/d.compactionConcurrency)
	ordered := make([]forceChunk, 0, len(chunks))
	for offset := range stride {
		for i := offset; i < len(chunks); i += stride {
			ordered = append(ordered, chunks[i])
		}
	}

	for _, chunk := range ordered {
		group.Go(func() error {
			plantMu.Lock()
			err := d.plantRewriteMarkers(chunk.bottomSmallest)
			if err == nil && len(chunk.bottomSmallest) > 0 {
				err = d.db.Flush()
			}
			plantMu.Unlock()
			if err != nil {
				return fmt.Errorf("planting rewrite markers: %w", err)
			}

			return d.db.Compact(ctx, chunk.start, chunk.end, false)
		})
	}

	return group.Wait()
}

func (d *DB) maxTableNum() (pebble.TableNum, error) {
	tables, err := d.db.SSTables()
	if err != nil {
		return 0, err
	}

	var maxNum pebble.TableNum
	for _, level := range tables {
		for i := range level {
			maxNum = max(maxNum, level[i].FileNum)
		}
	}
	return maxNum, nil
}

// tableUpToDate reports whether a table's properties already match the
// configured filter policy and compression, so a forced compaction can skip
// rewriting it. Skipping is disabled when either expectation is unknown.
func (d *DB) tableUpToDate(props *sstable.Properties) bool {
	return d.tableFilterPolicy != "" && d.tableCompression != "" && props != nil &&
		props.FilterPolicyName == d.tableFilterPolicy &&
		props.CompressionName == d.tableCompression
}

// keyRange is one table's inclusive user-key span.
type keyRange struct {
	start, end []byte
}

// staleTables returns the ranges of every stale table — numbered at or below
// baseline and not matching the configured table options — sorted by start
// key and split into bottom-level and upper-level tables.
func (d *DB) staleTables(baseline pebble.TableNum) (bottom, upper []keyRange, err error) {
	tables, err := d.db.SSTables(pebble.WithProperties())
	if err != nil {
		return nil, nil, err
	}

	bottomLevel := len(tables) - 1
	for level, files := range tables {
		for i := range files {
			if files[i].FileNum > baseline || d.tableUpToDate(files[i].Properties) {
				continue
			}
			stale := keyRange{
				start: bytes.Clone(files[i].Smallest.UserKey),
				end:   bytes.Clone(files[i].Largest.UserKey),
			}
			if level == bottomLevel {
				bottom = append(bottom, stale)
			} else {
				upper = append(upper, stale)
			}
		}
	}
	byStart := func(a, b keyRange) int { return bytes.Compare(a.start, b.start) }
	slices.SortFunc(bottom, byStart)
	slices.SortFunc(upper, byStart)
	return bottom, upper, nil
}

// gapChunks returns extra chunks for the stale upper-level tables overlapping
// no chunk. Tables that overlap one need no extra chunk: the first chunk
// compaction to reach an upper level consumes every overlapping table there
// whole. The rest sit in a single inter-chunk gap each; tables in the same
// gap merge into one chunk — two chunks compacting overlapping ranges would
// conflict in pebble and stall the whole manual queue.
func gapChunks(chunks []forceChunk, upper []keyRange) []forceChunk {
	overlapsChunk := func(t keyRange) bool {
		next, _ := slices.BinarySearchFunc(chunks, t, func(c forceChunk, t keyRange) int {
			return bytes.Compare(c.start, t.start)
		})
		// Candidates: the chunk starting at or after t, and the one before it.
		return next < len(chunks) && bytes.Compare(chunks[next].start, t.end) <= 0 ||
			next > 0 && bytes.Compare(t.start, chunks[next-1].end) <= 0
	}

	var gaps []forceChunk
	for _, table := range upper {
		if overlapsChunk(table) {
			continue
		}
		if last := len(gaps) - 1; last >= 0 && bytes.Compare(table.start, gaps[last].end) <= 0 {
			if bytes.Compare(table.end, gaps[last].end) > 0 {
				gaps[last].end = table.end
			}
			continue
		}
		gaps = append(gaps, forceChunk{start: table.start, end: table.end})
	}
	return gaps
}

// staleChunks groups the stale bottom-level tables into chunks of
// forceCompactChunkTables tables. Bottom-level ranges are disjoint, so the
// chunks are too, and pebble can compact them concurrently. Stale upper-level
// tables ride along with the chunks they overlap, or get gap chunks of their
// own.
func (d *DB) staleChunks(baseline pebble.TableNum) ([]forceChunk, error) {
	bottom, upper, err := d.staleTables(baseline)
	if err != nil {
		return nil, err
	}

	var chunks []forceChunk
	for start := 0; start < len(bottom); start += forceCompactChunkTables {
		group := bottom[start:min(start+forceCompactChunkTables, len(bottom))]
		chunk := forceChunk{start: group[0].start, end: group[len(group)-1].end}
		for _, table := range group {
			chunk.bottomSmallest = append(chunk.bottomSmallest, table.start)
		}
		chunks = append(chunks, chunk)
	}
	chunks = append(chunks, gapChunks(chunks, upper)...)

	for i := range chunks {
		// Compact requires start < end; extend a single-key chunk minimally.
		if bytes.Equal(chunks[i].start, chunks[i].end) {
			chunks[i].end = append(bytes.Clone(chunks[i].end), 0)
		}
	}
	return chunks, nil
}

// plantRewriteMarkers overwrites each given bottom-level table's smallest key
// with its current merged value, or re-deletes it when a newer tombstone
// shadows it. Either marker changes no data, and its flushed sstable overlaps
// the table's range, forcing a following manual compaction to rewrite it.
func (d *DB) plantRewriteMarkers(smallestKeys [][]byte) error {
	for _, smallest := range smallestKeys {
		value, closer, err := d.db.Get(smallest)
		if errors.Is(err, pebble.ErrNotFound) {
			if err := d.db.Delete(smallest, pebble.NoSync); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}

		value = bytes.Clone(value) // closing the getter invalidates the value
		if err := closer.Close(); err != nil {
			return err
		}
		if err := d.db.Set(smallest, value, pebble.NoSync); err != nil {
			return err
		}
	}

	return nil
}

type Item struct {
	Count uint
	Size  db.DataSize
}

func (i *Item) add(size db.DataSize) {
	i.Count++
	i.Size += size
}

func CalculatePrefixSize(ctx context.Context, pDB *DB, prefix []byte, withUpperBound bool) (*Item, error) {
	var (
		err error
		v   []byte

		item = &Item{}
	)

	it, err := pDB.NewIterator(prefix, withUpperBound)
	if err != nil {
		return nil, err
	}

	for it.First(); it.Valid(); it.Next() {
		if ctx.Err() != nil {
			return item, errors.Join(ctx.Err(), it.Close())
		}
		v, err = it.Value()
		if err != nil {
			return nil, errors.Join(err, it.Close())
		}

		item.add(db.DataSize(len(it.Key()) + len(v)))
	}

	return item, errors.Join(err, it.Close())
}
