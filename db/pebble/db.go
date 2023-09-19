package pebble

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

var _ db.DB = (*DB)(nil)

type DB struct {
	pebble   *pebble.DB
	wMutex   *sync.Mutex
	listener db.EventListener
}

// New opens a new database at the given path
func New(path string, logger pebble.Logger) (db.DB, error) {
	pDB, err := newPebble(path, &pebble.Options{Logger: logger})
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
	// hookup into events
	if options.EventListener == nil {
		options.EventListener = &pebble.EventListener{}
	}
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
func (d *DB) NewTransaction(update bool) db.Transaction {
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

	return txn
}

// Close : see io.Closer.Close
func (d *DB) Close() error {
	return d.pebble.Close()
}

// View : see db.DB.View
func (d *DB) View(fn func(txn db.Transaction) error) error {
	txn := d.NewTransaction(false)
	defer discardTxnOnPanic(txn)
	return utils.RunAndWrapOnError(txn.Discard, fn(txn))
}

// Update : see db.DB.Update
func (d *DB) Update(fn func(txn db.Transaction) error) error {
	txn := d.NewTransaction(true)
	defer discardTxnOnPanic(txn)
	if err := fn(txn); err != nil {
		return utils.RunAndWrapOnError(txn.Discard, err)
	}
	return utils.RunAndWrapOnError(txn.Discard, txn.Commit())
}

// Impl : see db.DB.Impl
func (d *DB) Impl() any {
	return d.pebble
}

func discardTxnOnPanic(txn db.Transaction) {
	p := recover()
	if p != nil {
		if err := txn.Discard(); err != nil {
			fmt.Fprintf(os.Stderr, "failed discarding panicing txn err: %s", err)
		}
		panic(p)
	}
}
