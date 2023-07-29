package pebble

import (
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

var _ db.DB = (*DB)(nil)

type DB struct {
	pebble *pebble.DB
	wMutex *sync.Mutex
}

// New opens a new database at the given path
func New(path string, logger pebble.Logger) (db.DB, error) {
	return newPebble(path, &pebble.Options{
		Cache:  pebble.NewCache(32000000000),
		Logger: logger,
	})
}

// NewMem opens a new in-memory database
func NewMem() (db.DB, error) {
	return newPebble("", &pebble.Options{
		FS: vfs.NewMem(),
	})
}

// NewMemTest opens a new in-memory database, panics on error
func NewMemTest() db.DB {
	memDB, err := NewMem()
	if err != nil {
		panic(err)
	}
	return memDB
}

func newPebble(path string, options *pebble.Options) (db.DB, error) {
	pDB, err := pebble.Open(path, options)
	if err != nil {
		return nil, err
	}
	return &DB{pDB, new(sync.Mutex)}, nil
}

// NewTransaction : see db.DB.NewTransaction
func (d *DB) NewTransaction(update bool) db.Transaction {
	txn := &Transaction{}
	if update {
		d.wMutex.Lock()
		txn.lock = d.wMutex
		txn.batch = d.pebble.NewIndexedBatch()
	} else {
		txn.snapshot = d.pebble.NewSnapshot()
	}

	return txn
}

// Close : see io.Closer.Close
func (d *DB) Close() error {
	return d.pebble.Close()
}

// View : see db.DB.View
func (d *DB) View(fn func(txn db.Transaction) error) error {
	txn := d.NewTransaction(false)
	return db.CloseAndWrapOnError(txn.Discard, fn(txn))
}

// View : see db.DB.View
func (d *DB) PersistedView() (db.Transaction, func() error, error) {
	txn := d.NewTransaction(false)
	return txn, func() error {
		return db.CloseAndWrapOnError(txn.Discard, nil)
	}, nil
}

// Update : see db.DB.Update
func (d *DB) Update(fn func(txn db.Transaction) error) error {
	txn := d.NewTransaction(true)
	if err := fn(txn); err != nil {
		return db.CloseAndWrapOnError(txn.Discard, err)
	}
	return db.CloseAndWrapOnError(txn.Discard, txn.Commit())
}

// Impl : see db.DB.Impl
func (d *DB) Impl() any {
	return d.pebble
}
