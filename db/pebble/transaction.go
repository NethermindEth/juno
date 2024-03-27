package pebble

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble"
)

var ErrDiscardedTransaction = errors.New("discarded txn")

var _ db.Transaction = (*Transaction)(nil)

type Transaction struct {
	batch    *pebble.Batch
	snapshot *pebble.Snapshot
	lock     *sync.Mutex
	listener db.EventListener
}

// Discard : see db.Transaction.Discard
func (t *Transaction) Discard() error {
	if t.batch != nil {
		if err := t.batch.Close(); err != nil {
			return err
		}
		t.batch = nil
	}
	if t.snapshot != nil {
		if err := t.snapshot.Close(); err != nil {
			return err
		}
		t.snapshot = nil
	}

	if t.lock != nil {
		t.lock.Unlock()
		t.lock = nil
	}
	return nil
}

// Commit : see db.Transaction.Commit
func (t *Transaction) Commit() error {
	start := time.Now()
	defer func() { t.listener.OnCommit(time.Since(start)) }()
	if t.batch != nil {
		return utils.RunAndWrapOnError(t.Discard, t.batch.Commit(pebble.Sync))
	}
	return utils.RunAndWrapOnError(t.Discard, ErrDiscardedTransaction)
}

// Set : see db.Transaction.Set
func (t *Transaction) Set(key, val []byte) error {
	start := time.Now()
	if t.batch == nil {
		return errors.New("read only transaction")
	}
	if len(key) == 0 {
		return errors.New("empty key")
	}

	defer func() { t.listener.OnIO(true, time.Since(start)) }()
	return t.batch.Set(key, val, pebble.Sync)
}

// Delete : see db.Transaction.Delete
func (t *Transaction) Delete(key []byte) error {
	start := time.Now()
	if t.batch == nil {
		return errors.New("read only transaction")
	}

	defer func() { t.listener.OnIO(true, time.Since(start)) }()
	return t.batch.Delete(key, pebble.Sync)
}

// Get : see db.Transaction.Get
func (t *Transaction) Get(key []byte, cb func([]byte) error) error {
	start := time.Now()
	var val []byte
	var closer io.Closer

	var err error
	if t.batch != nil {
		val, closer, err = t.batch.Get(key)
	} else if t.snapshot != nil {
		val, closer, err = t.snapshot.Get(key)
	} else {
		return ErrDiscardedTransaction
	}

	defer t.listener.OnIO(false, time.Since(start)) //nolint:govet
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return db.ErrKeyNotFound
		}

		return err
	}
	return utils.RunAndWrapOnError(closer.Close, cb(val))
}

// Impl : see db.Transaction.Impl
func (t *Transaction) Impl() any {
	if t.batch != nil {
		return t.batch
	}

	if t.snapshot != nil {
		return t.snapshot
	}
	return nil
}

// NewIterator : see db.Transaction.NewIterator
func (t *Transaction) NewIterator() (db.Iterator, error) {
	var iter *pebble.Iterator
	var err error
	if t.batch != nil {
		iter, err = t.batch.NewIter(nil)
	} else if t.snapshot != nil {
		iter, err = t.snapshot.NewIter(nil)
	} else {
		return nil, ErrDiscardedTransaction
	}

	if err != nil {
		return nil, err
	}

	return &iterator{iter: iter}, nil
}
