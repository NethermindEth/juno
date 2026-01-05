package pebblev2

import (
	"errors"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/cockroachdb/pebble/v2"
)

var _ db.Snapshot = (*snapshot)(nil)

type snapshot struct {
	snapshot *pebble.Snapshot
	listener db.EventListener
}

func NewSnapshot(db *pebble.DB, listener db.EventListener) *snapshot {
	return &snapshot{snapshot: db.NewSnapshot(), listener: listener}
}

func (s *snapshot) Has(key []byte) (bool, error) {
	_, closer, err := s.snapshot.Get(key)
	if err != nil {
		return false, err
	}

	return true, closer.Close()
}

func (s *snapshot) Get(key []byte, cb func(value []byte) error) error {
	start := time.Now()

	defer func() {
		s.listener.OnIO(false, time.Since(start))
	}()
	data, closer, err := s.snapshot.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return db.ErrKeyNotFound
		}
		return err
	}

	if err := cb(data); err != nil {
		return err
	}

	return closer.Close()
}

func (s *snapshot) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
	iterOpt := &pebble.IterOptions{LowerBound: prefix}
	if withUpperBound {
		iterOpt.UpperBound = dbutils.UpperBound(prefix)
	}

	it, err := s.snapshot.NewIter(iterOpt)
	if err != nil {
		return nil, err
	}

	return &iterator{iter: it}, nil
}

func (s *snapshot) Close() error {
	return s.snapshot.Close()
}
