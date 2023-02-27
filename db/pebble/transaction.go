package pebble

import (
	"errors"
	"io"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
)

var ErrDiscardedTransaction = errors.New("discarded txn")

type Transaction struct {
	batch    *pebble.Batch
	snapshot *pebble.Snapshot
	lock     *sync.Mutex
}

// Discard : see db.Transaction.Discard
func (t *Transaction) Discard() (err error) {
	if t.batch != nil {
		err = t.batch.Close()
		t.batch = nil
	}
	if t.snapshot != nil {
		err = t.snapshot.Close()
		t.snapshot = nil
	}

	if t.lock != nil {
		t.lock.Unlock()
		t.lock = nil
	}
	return
}

// Commit : see db.Transaction.Commit
func (t *Transaction) Commit() (err error) {
	defer db.CloseAndWrapOnError(t.Discard, &err)

	if t.batch != nil {
		return t.batch.Commit(pebble.Sync)
	} else {
		return ErrDiscardedTransaction
	}
}

// Set : see db.Transaction.Set
func (t *Transaction) Set(key, val []byte) error {
	if t.batch == nil {
		return errors.New("read only transaction")
	} else if len(key) == 0 {
		return errors.New("empty key")
	}
	return t.batch.Set(key, val, pebble.Sync)
}

// Delete : see db.Transaction.Delete
func (t *Transaction) Delete(key []byte) error {
	if t.batch == nil {
		return errors.New("read only transaction")
	}
	return t.batch.Delete(key, pebble.Sync)
}

// Get : see db.Transaction.Get
func (t *Transaction) Get(key []byte, cb func([]byte) error) (err error) {
	var val []byte
	var closer io.Closer

	if t.batch != nil {
		val, closer, err = t.batch.Get(key)
	} else if t.snapshot != nil {
		val, closer, err = t.snapshot.Get(key)
	} else {
		return ErrDiscardedTransaction
	}
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return db.ErrKeyNotFound
		}

		return
	}
	defer db.CloseAndWrapOnError(closer.Close, &err)
	return cb(val)
}

// Impl : see db.Transaction.Impl
func (t *Transaction) Impl() any {
	if t.batch != nil {
		return t.batch
	} else if t.snapshot != nil {
		return t.snapshot
	} else {
		return nil
	}
}

// NewIterator : see db.Transaction.NewIterator
func (t *Transaction) NewIterator() (db.Iterator, error) {
	var iter *pebble.Iterator
	if t.batch != nil {
		iter = t.batch.NewIter(nil)
	} else if t.snapshot != nil {
		iter = t.snapshot.NewIter(nil)
	} else {
		return nil, ErrDiscardedTransaction
	}

	return &iterator{iter: iter}, nil
}
