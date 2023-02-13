package db

import (
	"errors"
	"io"
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

type pebbleDb struct {
	pebble *pebble.DB
	wMutex *sync.Mutex
}

type pebbleTxn struct {
	batch    *pebble.Batch
	snapshot *pebble.Snapshot
	lock     *sync.Mutex
}

// NewTransaction : see db.DB.NewTransaction
func (db *pebbleDb) NewTransaction(update bool) Transaction {
	txn := &pebbleTxn{}
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
func (db *pebbleDb) Close() error {
	return db.pebble.Close()
}

// View : see db.DB.View
func (db *pebbleDb) View(fn func(txn Transaction) error) error {
	txn := db.NewTransaction(false)
	defer txn.Discard()

	return fn(txn)
}

// Update : see db.DB.Update
func (db *pebbleDb) Update(fn func(txn Transaction) error) error {
	txn := db.NewTransaction(true)
	defer txn.Discard()

	if err := fn(txn); err != nil {
		return err
	}

	return txn.Commit()
}

// Impl : see db.DB.Impl
func (db *pebbleDb) Impl() any {
	return db.pebble
}

// Discard : see db.Transaction.Discard
func (t *pebbleTxn) Discard() {
	if t.batch != nil {
		t.batch.Close()
		t.batch = nil
	}
	if t.snapshot != nil {
		t.snapshot.Close()
		t.snapshot = nil
	}

	if t.lock != nil {
		t.lock.Unlock()
		t.lock = nil
	}
}

// Commit : see db.Transaction.Commit
func (t *pebbleTxn) Commit() error {
	defer t.Discard()
	if t.batch != nil {
		return t.batch.Commit(pebble.Sync)
	} else {
		return errors.New("discarded txn")
	}
}

// Set : see db.Transaction.Set
func (t *pebbleTxn) Set(key, val []byte) error {
	if t.batch == nil {
		return errors.New("read only transaction")
	} else if len(key) == 0 {
		return errors.New("empty key")
	}
	return t.batch.Set(key, val, pebble.Sync)
}

// Delete : see db.Transaction.Delete
func (t *pebbleTxn) Delete(key []byte) error {
	if t.batch == nil {
		return errors.New("read only transaction")
	}
	return t.batch.Delete(key, pebble.Sync)
}

// Get : see db.Transaction.Get
func (t *pebbleTxn) Get(key []byte, cb func([]byte) error) error {
	var val []byte
	var closer io.Closer
	var err error
	if t.batch != nil {
		val, closer, err = t.batch.Get(key)
	} else if t.snapshot != nil {
		val, closer, err = t.snapshot.Get(key)
	} else {
		return errors.New("discarded txn")
	}
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return ErrKeyNotFound
		}

		return err
	}
	defer closer.Close()
	return cb(val)
}

// Seek : see db.Transaction.Seek
func (t *pebbleTxn) Seek(key []byte, cb func(*Entry) error) error {
	var iter *pebble.Iterator
	if t.batch != nil {
		iter = t.batch.NewIter(&pebble.IterOptions{
			LowerBound: key,
		})
	} else if t.snapshot != nil {
		iter = t.snapshot.NewIter(&pebble.IterOptions{
			LowerBound: key,
		})
	} else {
		return errors.New("discarded txn")
	}
	defer iter.Close()

	if iter.Valid() {
		if val, err := iter.ValueAndErr(); err != nil {
			return err
		} else {
			return cb(&Entry{
				Key:   iter.Key(),
				Value: val,
			})
		}
	}

	return nil
}

// Impl : see db.Transaction.Impl
func (t *pebbleTxn) Impl() any {
	if t.batch != nil {
		return t.batch
	} else if t.snapshot != nil {
		return t.snapshot
	} else {
		return nil
	}
}

// NewDb opens a new database at the given path
func NewDb(path string, logger pebble.Logger) (DB, error) {
	return newPebble(path, &pebble.Options{
		Logger: logger,
	})
}

// NewInMemoryDb opens a new in-memory database
func NewInMemoryDb() (DB, error) {
	return newPebble("", &pebble.Options{
		FS: vfs.NewMem(),
	})
}

func newPebble(path string, options *pebble.Options) (DB, error) {
	if pDb, err := pebble.Open(path, options); err != nil {
		return nil, err
	} else {
		return &pebbleDb{pDb, new(sync.Mutex)}, nil
	}
}

// NewTestDb opens a new in-memory database, panics on error
func NewTestDb() DB {
	db, err := NewInMemoryDb()
	if err != nil {
		panic(err)
	}
	return db
}
