package pebble

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/db"
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
var _ db.DB = (*DB)(nil)

type DB struct {
	pebble   *pebble.DB
	wMutex   *sync.Mutex
	listener db.EventListener
}

// New opens a new database at the given path with default options
func New(path string) (db.DB, error) {
	return newPebble(path, nil)
}

func NewWithOptions(path string, cacheSizeMB uint, maxOpenFiles int, colouredLogger bool) (db.DB, error) {
	// Ensure that the specified cache size meets a minimum threshold.
	cacheSizeMB = max(cacheSizeMB, minCacheSizeMB)

	dbLog, err := utils.NewZapLogger(utils.ERROR, colouredLogger)
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
func NewMemTest(t *testing.T) db.DB {
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
	return &DB{pebble: pDB, wMutex: new(sync.Mutex), listener: &db.SelectiveListener{}}, nil
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
