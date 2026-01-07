package pebble

import (
	"errors"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/cockroachdb/pebble"
)

var _ db.Batch = (*batch)(nil)

type batch struct {
	batch    *pebble.Batch
	db       *DB
	size     int // size of the batch in bytes
	listener db.EventListener
}

func NewBatch(dbBatch *pebble.Batch, db *DB, listener db.EventListener) *batch {
	return &batch{
		batch:    dbBatch,
		db:       db,
		listener: listener,
	}
}

// Delete : see db.Transaction.Delete
func (b *batch) Delete(key []byte) error {
	if b.batch == nil {
		return pebble.ErrClosed
	}
	defer b.listener.OnIO(true, time.Now())

	if err := b.batch.Delete(key, pebble.Sync); err != nil {
		return err
	}
	b.size += len(key)
	return nil
}

func (b *batch) DeleteRange(start, end []byte) error {
	if b.batch == nil {
		return pebble.ErrClosed
	}
	defer b.listener.OnIO(true, time.Now())

	return b.batch.DeleteRange(start, end, pebble.Sync)
}

func (b *batch) Get(key []byte, cb func(value []byte) error) error {
	if b.batch == nil {
		return pebble.ErrClosed
	}
	defer b.listener.OnIO(false, time.Now())

	val, closer, err := b.batch.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return db.ErrKeyNotFound
		}
		return err
	}

	if err := cb(val); err != nil {
		return err
	}

	return closer.Close()
}

func (b *batch) Has(key []byte) (bool, error) {
	if b.batch == nil {
		return false, pebble.ErrClosed
	}
	defer b.listener.OnIO(false, time.Now())

	_, closer, err := b.batch.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, closer.Close()
}

func (b *batch) NewIterator(lowerBound []byte, withUpperBound bool) (db.Iterator, error) {
	if b.batch == nil {
		return nil, pebble.ErrClosed
	}

	var iter *pebble.Iterator
	var err error

	iterOpt := &pebble.IterOptions{LowerBound: lowerBound}
	if withUpperBound {
		iterOpt.UpperBound = dbutils.UpperBound(lowerBound)
	}

	iter, err = b.batch.NewIter(iterOpt)
	if err != nil {
		return nil, err
	}

	return &iterator{iter: iter, listener: b.listener}, nil
}

func (b *batch) Put(key, value []byte) error {
	if b.batch == nil {
		return pebble.ErrClosed
	}
	defer b.listener.OnIO(true, time.Now())

	if err := b.batch.Set(key, value, pebble.Sync); err != nil {
		return err
	}
	b.size += len(key) + len(value)
	return nil
}

func (b *batch) Size() int {
	return b.size
}

func (b *batch) Write() error {
	if b.batch == nil {
		return pebble.ErrClosed
	}
	defer b.listener.OnCommit(time.Now())

	b.db.closeLock.RLock()
	defer b.db.closeLock.RUnlock()

	if b.db.closed {
		return pebble.ErrClosed
	}

	if err := b.batch.Commit(pebble.Sync); err != nil {
		return err
	}

	return b.Close()
}

func (b *batch) Close() error {
	if b.batch == nil {
		return pebble.ErrClosed
	}

	if err := b.batch.Close(); err != nil {
		return err
	}

	// Clear all the fields to prevent any further use of the batch.
	*b = batch{}
	return nil
}
