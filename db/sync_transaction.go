package db

import (
	"errors"
	"sync"
)

type SyncTransaction struct {
	lock sync.RWMutex
	txn  Transaction
}

func NewSyncTransaction(txn Transaction) *SyncTransaction {
	syncTxn, ok := txn.(*SyncTransaction)
	if ok {
		return syncTxn
	}
	return &SyncTransaction{
		txn: txn,
	}
}

// Discard : see db.Transaction.Discard
func (t *SyncTransaction) Discard() error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.txn.Discard()
}

// Commit : see db.Transaction.Commit
func (t *SyncTransaction) Commit() error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.txn.Commit()
}

// Set : see db.Transaction.Set
func (t *SyncTransaction) Set(key, val []byte) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.txn.Set(key, val)
}

// Delete : see db.Transaction.Delete
func (t *SyncTransaction) Delete(key []byte) error {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.txn.Delete(key)
}

// Get : see db.Transaction.Get
func (t *SyncTransaction) Get(key []byte, cb func([]byte) error) error {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.txn.Get(key, cb)
}

// Impl : see db.Transaction.Impl
func (t *SyncTransaction) Impl() any {
	return t.txn
}

// NewIterator : see db.Transaction.NewIterator
func (t *SyncTransaction) NewIterator() (Iterator, error) {
	return nil, errors.New("sync transactions dont support iterators")
}
