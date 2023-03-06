package pebble

import (
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

type DB struct {
	pebble *pebble.DB
	wMutex *sync.Mutex
}

// New opens a new database at the given path
func New(path string, logger pebble.Logger) (db.DB, error) {
	return newPebble(path, &pebble.Options{
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
	db, err := NewMem()
	if err != nil {
		panic(err)
	}
	return db
}

func newPebble(path string, options *pebble.Options) (db.DB, error) {
	if pDb, err := pebble.Open(path, options); err != nil {
		return nil, err
	} else {
		return &DB{pDb, new(sync.Mutex)}, nil
	}
}

// NewTransaction : see db.DB.NewTransaction
func (db *DB) NewTransaction(update bool) db.Transaction {
	txn := &Transaction{}
	if update {
		db.wMutex.Lock()
		txn.lock = db.wMutex
		txn.batch = db.pebble.NewIndexedBatch()
	} else {
		txn.snapshot = db.pebble.NewSnapshot()
	}

	return txn
}

// Close : see io.Closer.Close
func (db *DB) Close() error {
	return db.pebble.Close()
}

// View : see db.DB.View
func (d *DB) View(fn func(txn db.Transaction) error) (err error) {
	txn := d.NewTransaction(false)
	defer db.CloseAndWrapOnError(txn.Discard, &err)

	return fn(txn)
}

// Update : see db.DB.Update
func (d *DB) Update(fn func(txn db.Transaction) error) (err error) {
	txn := d.NewTransaction(true)
	defer db.CloseAndWrapOnError(txn.Discard, &err)

	if err = fn(txn); err != nil {
		return err
	}

	return txn.Commit()
}

// Impl : see db.DB.Impl
func (db *DB) Impl() any {
	return db.pebble
}
