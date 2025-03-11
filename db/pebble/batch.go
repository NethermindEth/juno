package pebble

import (
	"errors"
	"sync"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble"
)

var (
	_ db.Transaction = (*batch)(nil)
	_ db.Batch       = (*batch)(nil)
)

type batch struct {
	batch    *pebble.Batch
	db       *DB
	size     int
	lock     *sync.RWMutex
	listener db.EventListener
}

func NewBatch(dbBatch *pebble.Batch, db *DB, lock *sync.RWMutex, listener db.EventListener) *batch {
	return &batch{
		batch:    dbBatch,
		db:       db,
		lock:     lock,
		listener: listener,
	}
}

// Discard : see db.Transaction.Discard
func (b *batch) Discard() error {
	if b.batch == nil {
		return nil
	}

	err := b.batch.Close()
	b.batch = nil
	b.lock.Unlock()
	b.lock = nil

	return err
}

// Commit : see db.Transaction.Commit
func (b *batch) Commit() error {
	if b.batch == nil {
		return ErrDiscardedTransaction
	}

	start := time.Now()
	defer func() { b.listener.OnCommit(time.Since(start)) }()
	return utils.RunAndWrapOnError(b.Discard, b.batch.Commit(pebble.Sync))
}

// Set : see db.Transaction.Set
func (b *batch) Set(key, val []byte) error {
	start := time.Now()
	if len(key) == 0 {
		return errors.New("empty key")
	}

	if b.batch == nil {
		return ErrDiscardedTransaction
	}

	defer func() { b.listener.OnIO(true, time.Since(start)) }()

	return b.batch.Set(key, val, pebble.Sync)
}

// Delete : see db.Transaction.Delete
func (b *batch) Delete(key []byte) error {
	if b.batch == nil {
		return ErrDiscardedTransaction
	}

	start := time.Now()
	defer func() { b.listener.OnIO(true, time.Since(start)) }()

	if err := b.batch.Delete(key, pebble.Sync); err != nil {
		return err
	}
	b.size += len(key)
	return nil
}

// Get : see db.Transaction.Get
func (b *batch) Get(key []byte, cb func([]byte) error) error {
	if b.batch == nil {
		return ErrDiscardedTransaction
	}
	return get(b.batch, key, cb, b.listener)
}

func (b *batch) Get2(key []byte) ([]byte, error) {
	if b.batch == nil {
		return nil, ErrDiscardedTransaction
	}

	start := time.Now()
	defer func() { b.listener.OnIO(false, time.Since(start)) }()

	val, closer, err := b.batch.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, db.ErrKeyNotFound
		}
		return nil, err
	}

	return utils.CopySlice(val), closer.Close()
}

func (b *batch) Has(key []byte) (bool, error) {
	if b.batch == nil {
		return false, ErrDiscardedTransaction
	}

	_, closer, err := b.batch.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, closer.Close()
}

// NewIterator : see db.Transaction.NewIterator
func (b *batch) NewIterator(lowerBound []byte, withUpperBound bool) (db.Iterator, error) {
	var iter *pebble.Iterator
	var err error

	if b.batch == nil {
		return nil, ErrDiscardedTransaction
	}

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
	if b.batch == nil {
		return ErrDiscardedTransaction
	}

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
	if b.db.closed {
		return pebble.ErrClosed
	}
	return b.batch.Commit(pebble.Sync)
}

func (b *batch) Reset() {
	b.batch.Reset()
	b.size = 0
}
