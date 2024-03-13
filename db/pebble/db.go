package pebble

import (
	"fmt"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

const (
	// minCache is the minimum amount of memory in megabytes to allocate to pebble read and write caching.
	minCache = 8

	megabyte = 1 << 20
)

var _ db.DB = (*DB)(nil)

type DB struct {
	pebble   *pebble.DB
	wMutex   *sync.Mutex
	listener db.EventListener
}

// New opens a new database at the given path
func New(path string, cache uint, maxOpenFiles int, logger pebble.Logger) (db.DB, error) {
	// Ensure that the specified cache size meets a minimum threshold.
	if cache < minCache {
		cache = minCache
	}
	pDB, err := newPebble(path, &pebble.Options{
		Logger:       logger,
		Cache:        pebble.NewCache(int64(cache * megabyte)),
		MaxOpenFiles: maxOpenFiles,
	})
	if err != nil {
		return nil, err
	}
	return pDB, nil
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
func (d *DB) NewTransaction(update bool) (db.Transaction, error) {
	txn := &Transaction{
		listener: d.listener,
	}
	if update {
		d.wMutex.Lock()
		txn.lock = d.wMutex
		txn.batch = d.pebble.NewIndexedBatch()
	} else {
		txn.snapshot = d.pebble.NewSnapshot()
	}

	return txn, nil
}

// Close : see io.Closer.Close
func (d *DB) Close() error {
	fmt.Println("PEBBLE CLOSED")
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
