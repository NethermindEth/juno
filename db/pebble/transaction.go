package pebble

import (
	"errors"
	"io"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
)

type Transaction struct {
	batch    *pebble.Batch
	snapshot *pebble.Snapshot
	lock     *sync.Mutex
}

type pebbleIterator struct {
	iter *pebble.Iterator
}

// Discard : see db.Transaction.Discard
func (t *Transaction) Discard() {
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
func (t *Transaction) Commit() error {
	defer t.Discard()
	if t.batch != nil {
		return t.batch.Commit(pebble.Sync)
	} else {
		return errors.New("discarded txn")
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
func (t *Transaction) Get(key []byte, cb func([]byte) error) error {
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
			return db.ErrKeyNotFound
		}

		return err
	}
	defer closer.Close()
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
func (t *Transaction) NewIterator(opts db.IterOptions) db.Iterator {
	var iter *pebble.Iterator
	if t.batch != nil {
		iter = t.batch.NewIter(&pebble.IterOptions{
			LowerBound: opts.LowerBound,
			UpperBound: opts.UpperBound,
		})
	} else if t.snapshot != nil {
		iter = t.snapshot.NewIter(&pebble.IterOptions{
			LowerBound: opts.LowerBound,
			UpperBound: opts.UpperBound,
		})
	} else {
		return nil
	}

	return &pebbleIterator{
		iter: iter,
	}
}

// Valid : see db.Transaction.Iterator.Valid
func (it *pebbleIterator) Valid() bool {
	return it.iter.Valid()
}

// Key : see db.Transaction.Iterator.Key
func (it *pebbleIterator) Key() []byte {
	return it.iter.Key()
}

// Value : see db.Transaction.Iterator.Value
func (it *pebbleIterator) Value() []byte {
	return it.iter.Value()
}

// Next : see db.Transaction.Iterator.Next
func (it *pebbleIterator) Next() bool {
	return it.iter.Next()
}

// Seek : see db.Transaction.Iterator.Seek
func (it *pebbleIterator) Seek(cb func(*db.Entry) error) error {
	for it.iter.First(); it.iter.Valid(); it.iter.Next() {
		val, err := it.iter.ValueAndErr()
		if err != nil {
			return err
		}
		if err := cb(&db.Entry{
			Key:   it.iter.Key(),
			Value: val,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Close : see db.Transaction.Iterator.Close
func (it *pebbleIterator) Close() error {
	it.iter.Close()
	return nil
}
