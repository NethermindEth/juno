package pebble

import (
	"errors"
	"sync"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/cockroachdb/pebble"
)

var _ db.Transaction = (*batch)(nil)

type batch struct {
	batch    *pebble.Batch
	dbLock   *sync.Mutex
	rwlock   sync.RWMutex
	listener db.EventListener
}

func NewBatch(dbBatch *pebble.Batch, dbLock *sync.Mutex, listener db.EventListener) *batch {
	return &batch{
		batch:    dbBatch,
		dbLock:   dbLock,
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
	b.dbLock.Unlock()
	b.dbLock = nil

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
	b.rwlock.Lock()
	defer b.rwlock.Unlock()

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
	b.rwlock.Lock()
	defer b.rwlock.Unlock()

	if b.batch == nil {
		return ErrDiscardedTransaction
	}

	start := time.Now()
	defer func() { b.listener.OnIO(true, time.Since(start)) }()

	return b.batch.Delete(key, pebble.Sync)
}

// Get : see db.Transaction.Get
func (b *batch) Get(key []byte, cb func([]byte) error) error {
	b.rwlock.RLock()
	defer b.rwlock.RUnlock()

	if b.batch == nil {
		return ErrDiscardedTransaction
	}
	return get(b.batch, key, cb, b.listener)
}

// NewIterator : see db.Transaction.NewIterator
func (b *batch) NewIterator(opts db.IterOptions) (db.Iterator, error) {
	var iter *pebble.Iterator
	var err error

	if b.batch == nil {
		return nil, ErrDiscardedTransaction
	}

	iter, err = b.batch.NewIter(&pebble.IterOptions{
		LowerBound: opts.LowerBound,
		UpperBound: opts.UpperBound,
	})
	if err != nil {
		return nil, err
	}

	return &iterator{iter: iter}, nil
}
