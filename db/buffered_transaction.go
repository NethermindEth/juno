package db

import (
	"errors"
	"slices"
)

// BufferedTransaction buffers the updates in the memory to be later flushed to the underlying Transaction
type BufferedTransaction struct {
	updates map[string][]byte
	txn     Transaction
}

func NewBufferedTransaction(txn Transaction) *BufferedTransaction {
	return &BufferedTransaction{
		txn:     txn,
		updates: make(map[string][]byte),
	}
}

// Discard : see db.Transaction.Discard
func (t *BufferedTransaction) Discard() error {
	t.updates = nil
	return t.txn.Discard()
}

// Commit : see db.Transaction.Commit
func (t *BufferedTransaction) Commit() error {
	if err := t.Flush(); err != nil {
		return err
	}
	t.updates = nil
	return t.txn.Commit()
}

// Set : see db.Transaction.Set
func (t *BufferedTransaction) Set(key, val []byte) error {
	valueCopy := make([]byte, 0, len(val))
	t.updates[string(key)] = slices.Concat(valueCopy, val)
	return nil
}

// Delete : see db.Transaction.Delete
func (t *BufferedTransaction) Delete(key []byte) error {
	t.updates[string(key)] = nil
	return nil
}

// Get : see db.Transaction.Get
func (t *BufferedTransaction) Get(key []byte, cb func([]byte) error) error {
	if value, found := t.updates[string(key)]; found {
		if value == nil {
			return ErrKeyNotFound
		}
		return cb(value)
	}
	return t.txn.Get(key, cb)
}

// Flush applies the pending changes to the underlying Transaction
func (t *BufferedTransaction) Flush() error {
	for key, value := range t.updates {
		keyBytes := []byte(key)
		if value == nil {
			if err := t.txn.Delete(keyBytes); err != nil {
				return err
			}
		} else {
			if err := t.txn.Set(keyBytes, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// Impl : see db.Transaction.Impl
func (t *BufferedTransaction) Impl() any {
	return t.txn
}

// NewIterator : see db.Transaction.NewIterator
func (t *BufferedTransaction) NewIterator() (Iterator, error) {
	return nil, errors.New("buffered transactions dont support iterators")
}
