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
	lock     *sync.Mutex
	listener db.EventListener
}

func NewBatch(dbBatch *pebble.Batch, lock *sync.Mutex, listener db.EventListener) *batch {
	return &batch{
		batch:    dbBatch,
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

	return b.batch.Delete(key, pebble.Sync)
}

// Get : see db.Transaction.Get
func (b *batch) Get(key []byte, cb func([]byte) error) error {
	if b.batch == nil {
		return ErrDiscardedTransaction
	}
	return get(b.batch, key, cb, b.listener)
}

// NewIterator : see db.Transaction.NewIterator
func (b *batch) NewIterator() (db.Iterator, error) {
	var iter *pebble.Iterator
	var err error

	if b.batch == nil {
		return nil, ErrDiscardedTransaction
	}

	iter, err = b.batch.NewIter(nil)
	if err != nil {
		return nil, err
	}

	return &iterator{iter: iter}, nil
}
