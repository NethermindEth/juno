package pebble

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

const (
	// minCache is the minimum amount of memory in megabytes to allocate to pebble read and write caching.
	// This is also pebble's default value.
	minCacheSizeMB = 8
)

var (
	ErrDiscardedTransaction = errors.New("discarded transaction")
	ErrReadOnlyTransaction  = errors.New("read-only transaction")
)

var (
	_ db.DB            = (*DB)(nil)
	_ db.KeyValueStore = (*DB)(nil)
)

// TODO(weiihann): check if lock is necessary
type DB struct {
	db       *pebble.DB
	closed   bool
	writeOpt *pebble.WriteOptions
	listener db.EventListener
	lock     *sync.RWMutex
}

// New opens a new database at the given path with default options
func New(path string) (db.DB, error) {
	return newPebble(path, nil)
}

func New2(path string) (db.KeyValueStore, error) {
	return newPebble(path, nil)
}

func NewWithOptions(path string, cacheSizeMB uint, maxOpenFiles int, colouredLogger bool) (db.DB, error) {
	// Ensure that the specified cache size meets a minimum threshold.
	cacheSizeMB = max(cacheSizeMB, minCacheSizeMB)
	log := utils.NewLogLevel(utils.ERROR)
	dbLog, err := utils.NewZapLogger(log, colouredLogger)
	if err != nil {
		return nil, fmt.Errorf("create DB logger: %w", err)
	}

	return newPebble(path, &pebble.Options{
		Logger:       dbLog,
		Cache:        pebble.NewCache(int64(cacheSizeMB * utils.Megabyte)),
		MaxOpenFiles: maxOpenFiles,
	})
}

// NewMem opens a new in-memory database
func NewMem() (db.DB, error) {
	return newPebble("", &pebble.Options{
		FS: vfs.NewMem(),
	})
}

// NewMemTest opens a new in-memory database, panics on error
func NewMemTest(t testing.TB) db.DB {
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

func newPebble(path string, options *pebble.Options) (*DB, error) {
	pDB, err := pebble.Open(path, options)
	if err != nil {
		return nil, err
	}
	return &DB{
		db:       pDB,
		lock:     new(sync.RWMutex),
		listener: &db.SelectiveListener{},
		writeOpt: &pebble.WriteOptions{Sync: true}, // TODO: can we use non-sync writes for performance?
	}, nil
}

// WithListener registers an EventListener
func (d *DB) WithListener(listener db.EventListener) db.DB {
	d.listener = listener
	return d
}

// NewTransaction : see db.DB.NewTransaction
// Batch is used for read-write operations, while snapshot is used for read-only operations
func (d *DB) NewTransaction(update bool) (db.Transaction, error) {
	if update {
		d.lock.Lock()
		return NewBatch(d.db.NewIndexedBatch(), d, d.lock, d.listener), nil
	}

	return NewSnapshot(d.db.NewSnapshot(), d.listener), nil
}

// Close : see io.Closer.Close
func (d *DB) Close() error {
	if d.closed {
		return nil
	}
	d.closed = true

	return d.db.Close()
}

// View : see db.DB.View
func (d *DB) View(fn func(txn db.Transaction) error) error {
	return db.View(d, fn)
}

// Update : see db.DB.Update
func (d *DB) Update(fn func(txn db.Transaction) error) error {
	return db.Update(d, fn)
}

func (d *DB) Update2(fn func(w db.IndexedBatch) error) error {
	if d.closed {
		return pebble.ErrClosed
	}

	batch := d.NewIndexedBatch()
	if err := fn(batch); err != nil {
		return err
	}

	return batch.Write()
}

func (d *DB) View2(fn func(r db.Snapshot) error) error {
	snap := d.NewSnapshot()
	return fn(snap)
}

func (d *DB) WithListener2(listener db.EventListener) db.KeyValueStore {
	d.listener = listener
	return d
}

// Impl : see db.DB.Impl
func (d *DB) Impl() any {
	return d.db
}

func (d *DB) Has(key []byte) (bool, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

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

	return true, utils.RunAndWrapOnError(closer.Close, err)
}

func (d *DB) Get2(key []byte) ([]byte, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()

	if d.closed {
		return nil, pebble.ErrClosed
	}

	val, closer, err := d.db.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, db.ErrKeyNotFound
		}
		return nil, err
	}

	return utils.CopySlice(val), utils.RunAndWrapOnError(closer.Close, nil)
}

func (d *DB) Put(key, val []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.closed {
		return pebble.ErrClosed
	}

	return d.db.Set(key, val, d.writeOpt)
}

func (d *DB) Delete(key []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.closed {
		return pebble.ErrClosed
	}

	return d.db.Delete(key, d.writeOpt)
}

func (d *DB) DeleteRange(start, end []byte) error {
	d.lock.Lock()
	defer d.lock.Unlock()

	if d.closed {
		return pebble.ErrClosed
	}

	return d.db.DeleteRange(start, end, d.writeOpt)
}

func (d *DB) NewBatch() db.Batch {
	return NewBatch(d.db.NewBatch(), d, d.lock, d.listener)
}

func (d *DB) NewBatchWithSize(size int) db.Batch {
	return NewBatch(d.db.NewBatchWithSize(size), d, d.lock, d.listener)
}

func (d *DB) NewIndexedBatch() db.IndexedBatch {
	return NewBatch(d.db.NewIndexedBatch(), d, d.lock, d.listener)
}

func (d *DB) NewIndexedBatchWithSize(size int) db.IndexedBatch {
	return NewBatch(d.db.NewIndexedBatchWithSize(size), d, d.lock, d.listener)
}

func (d *DB) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
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

	return &iterator{iter: it}, nil
}

func (d *DB) NewSnapshot() db.Snapshot {
	return NewSnapshot2(d.db, d.listener)
}

type Item struct {
	Count uint
	Size  utils.DataSize
}

func (i *Item) add(size utils.DataSize) {
	i.Count++
	i.Size += size
}

func CalculatePrefixSize(ctx context.Context, pDB *DB, prefix []byte, withUpperBound bool) (*Item, error) {
	var (
		err error
		v   []byte

		item = &Item{}
	)

	pebbleDB := pDB.Impl().(*pebble.DB)
	iterOpt := &pebble.IterOptions{LowerBound: prefix}
	if withUpperBound {
		iterOpt.UpperBound = dbutils.UpperBound(prefix)
	}

	it, err := pebbleDB.NewIter(iterOpt)
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
