package pebble

import (
	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
)

var _ db.Transaction = (*snapshot)(nil)

type snapshot struct {
	snapshot *pebble.Snapshot
	listener db.EventListener
}

func NewSnapshot(dbSnapshot *pebble.Snapshot, listener db.EventListener) *snapshot {
	return &snapshot{
		snapshot: dbSnapshot,
		listener: listener,
	}
}

// Discard : see db.Transaction.Discard
func (s *snapshot) Discard() error {
	if s.snapshot == nil {
		return nil
	}

	if err := s.snapshot.Close(); err != nil {
		return err
	}

	s.snapshot = nil

	return nil
}

// Commit : see db.Transaction.Commit
func (s *snapshot) Commit() error {
	return ErrReadOnlyTransaction
}

// Set : see db.Transaction.Set
func (s *snapshot) Set(key, val []byte) error {
	return ErrReadOnlyTransaction
}

// Delete : see db.Transaction.Delete
func (s *snapshot) Delete(key []byte) error {
	return ErrReadOnlyTransaction
}

// Get : see db.Transaction.Get
func (s *snapshot) Get(key []byte, cb func([]byte) error) error {
	if s.snapshot == nil {
		return ErrDiscardedTransaction
	}
	return get(s.snapshot, key, cb, s.listener)
}

// NewIterator : see db.Transaction.NewIterator
func (s *snapshot) NewIterator() (db.Iterator, error) {
	var iter *pebble.Iterator
	var err error

	if s.snapshot == nil {
		return nil, ErrDiscardedTransaction
	}

	iter, err = s.snapshot.NewIter(nil)
	if err != nil {
		return nil, err
	}

	return &iterator{iter: iter}, nil
}
