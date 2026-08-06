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
// it, so one pass is not always enough. Force mode expects a database opened
// with WithOfflineCompaction — automatic compactions may otherwise merge the
// per-chunk marker sstables and serialize the chunk compactions.
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
// compactions conflict on it and serialize.
func (d *DB) compactChunks(ctx context.Context, chunks []forceChunk) error {
	var plantMu sync.Mutex
	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(d.compactionConcurrency)

	for _, chunk := range chunks {
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

// staleChunks groups every table numbered at or below baseline into
// disjoint-range chunks of about forceCompactChunkTables tables. Overlapping
// tables always share a chunk — concurrent manual compactions over
// overlapping ranges conflict in pebble and stall the whole manual queue —
// so a chunk grows past the target when a wide upper-level table spans many
// bottom-level ones.
func (d *DB) staleChunks(baseline pebble.TableNum) ([]forceChunk, error) {
	tables, err := d.db.SSTables()
	if err != nil {
		return nil, err
	}

	type interval struct {
		start, end []byte
		bottom     bool
	}
	var stale []interval
	bottomLevel := len(tables) - 1
	for level, files := range tables {
		for i := range files {
			if files[i].FileNum > baseline {
				continue
			}
			stale = append(stale, interval{
				start:  bytes.Clone(files[i].Smallest.UserKey),
				end:    bytes.Clone(files[i].Largest.UserKey),
				bottom: level == bottomLevel,
			})
		}
	}
	slices.SortFunc(stale, func(a, b interval) int { return bytes.Compare(a.start, b.start) })

	var chunks []forceChunk
	var chunk forceChunk
	var count int
	flush := func() {
		// Compact requires start < end; extend a single-key chunk minimally.
		if bytes.Equal(chunk.start, chunk.end) {
			chunk.end = append(bytes.Clone(chunk.end), 0)
		}
		chunks = append(chunks, chunk)
	}
	for _, table := range stale {
		if count >= forceCompactChunkTables && bytes.Compare(table.start, chunk.end) > 0 {
			flush()
			chunk, count = forceChunk{}, 0
		}
		if count == 0 {
			chunk.start, chunk.end = table.start, table.end
		} else if bytes.Compare(table.end, chunk.end) > 0 {
			chunk.end = table.end
		}
		if table.bottom {
			chunk.bottomSmallest = append(chunk.bottomSmallest, table.start)
		}
		count++
	}
	if count > 0 {
		flush()
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
