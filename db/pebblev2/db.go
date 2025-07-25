package pebblev2

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble/v2"
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

var _ db.KeyValueStore = (*DB)(nil)

type DB struct {
	db        *pebble.DB
	closed    bool
	writeOpt  *pebble.WriteOptions
	listener  db.EventListener
	closeLock *sync.RWMutex // Ensures that the database is closed correctly
}

// New opens a new database at the given path with default options
func New(path string) (db.KeyValueStore, error) {
	return newPebble(path, nil)
}

func NewWithOptions(path string, cacheSizeMB uint, maxOpenFiles int, colouredLogger bool) (db.KeyValueStore, error) {
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

func newPebble(path string, options *pebble.Options) (*DB, error) {
	pDB, err := pebble.Open(path, options)
	if err != nil {
		return nil, err
	}
	return &DB{
		db:        pDB,
		closeLock: new(sync.RWMutex),
		listener:  &db.SelectiveListener{},
		writeOpt:  &pebble.WriteOptions{Sync: true}, // TODO: can we use non-sync writes for performance?
	}, nil
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

func (d *DB) View(fn func(r db.Snapshot) error) error {
	snap := d.NewSnapshot()
	return fn(snap)
}

func (d *DB) WithListener(listener db.EventListener) db.KeyValueStore {
	d.listener = listener
	return d
}

func (d *DB) Impl() any {
	return d.db
}

func (d *DB) Has(key []byte) (bool, error) {
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

	return true, utils.RunAndWrapOnError(closer.Close, err)
}

func (d *DB) Get(key []byte, cb func(value []byte) error) error {
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

	if err := cb(val); err != nil {
		return err
	}

	return closer.Close()
}

func (d *DB) Put(key, val []byte) error {
	d.closeLock.RLock()
	defer d.closeLock.RUnlock()

	if d.closed {
		return pebble.ErrClosed
	}

	return d.db.Set(key, val, d.writeOpt)
}

func (d *DB) Delete(key []byte) error {
	d.closeLock.RLock()
	defer d.closeLock.RUnlock()

	if d.closed {
		return pebble.ErrClosed
	}

	return d.db.Delete(key, d.writeOpt)
}

func (d *DB) DeleteRange(start, end []byte) error {
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

	return &iterator{iter: it}, nil
}

func (d *DB) NewSnapshot() db.Snapshot {
	return NewSnapshot(d.db, d.listener)
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

	it, err := pDB.NewIterator(prefix, withUpperBound)
	if err != nil {
		return nil, err
	}

	for it.First(); it.Valid(); it.Next() {
		if ctx.Err() != nil {
			return item, utils.RunAndWrapOnError(it.Close, ctx.Err())
		}
		v, err = it.Value()
		if err != nil {
			return nil, utils.RunAndWrapOnError(it.Close, err)
		}

		item.add(utils.DataSize(len(it.Key()) + len(v)))
	}

	return item, utils.RunAndWrapOnError(it.Close, err)
}
