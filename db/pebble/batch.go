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
	start := time.Now()
	defer func() { b.listener.OnIO(true, time.Since(start)) }()

	if err := b.batch.Delete(key, pebble.Sync); err != nil {
		return err
	}
	b.size += len(key)
	return nil
}

func (b *batch) DeleteRange(start, end []byte) error {
	return b.batch.DeleteRange(start, end, pebble.Sync)
}

//nolint:dupl
func (b *batch) Get(key []byte, cb func(value []byte) error) error {
	start := time.Now()
	defer func() { b.listener.OnIO(false, time.Since(start)) }()

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

	return &iterator{iter: iter}, nil
}

func (b *batch) Put(key, value []byte) error {
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
	b.db.closeLock.RLock()
	defer b.db.closeLock.RUnlock()

	if b.db.closed {
		return pebble.ErrClosed
	}

	if err := b.batch.Commit(pebble.Sync); err != nil {
		return err
	}

	return b.batch.Close()
}

func (b *batch) Reset() {
	b.batch.Reset()
	b.size = 0
}

func (b *batch) Close() error {
	return b.batch.Close()
}
