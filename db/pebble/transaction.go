package pebble

import (
	"errors"
	"io"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
	"github.com/prometheus/client_golang/prometheus"
)

var ErrDiscardedTransaction = errors.New("discarded txn")

var _ db.Transaction = (*Transaction)(nil)

type Transaction struct {
	batch    *pebble.Batch
	snapshot *pebble.Snapshot
	lock     *sync.Mutex

	// metrics
	readCounter  prometheus.Counter
	writeCounter prometheus.Counter
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
	if t.batch != nil {
		return db.CloseAndWrapOnError(t.Discard, t.batch.Commit(pebble.Sync))
	}
	return db.CloseAndWrapOnError(t.Discard, ErrDiscardedTransaction)
}

// Set : see db.Transaction.Set
func (t *Transaction) Set(key, val []byte) error {
	if t.batch == nil {
		return errors.New("read only transaction")
	}
	if len(key) == 0 {
		return errors.New("empty key")
	}

	if t.writeCounter != nil {
		t.writeCounter.Inc()
	}
	return t.batch.Set(key, val, pebble.Sync)
}

// Delete : see db.Transaction.Delete
func (t *Transaction) Delete(key []byte) error {
	if t.batch == nil {
		return errors.New("read only transaction")
	}

	if t.writeCounter != nil {
		t.writeCounter.Inc()
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
		return ErrDiscardedTransaction
	}

	if t.readCounter != nil {
		t.readCounter.Inc()
	}
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return db.ErrKeyNotFound
		}

		return err
	}
	return db.CloseAndWrapOnError(closer.Close, cb(val))
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
	if t.batch != nil {
		iter = t.batch.NewIter(nil)
	} else if t.snapshot != nil {
		iter = t.snapshot.NewIter(nil)
	} else {
		return nil, ErrDiscardedTransaction
	}

	return &iterator{iter: iter}, nil
}
