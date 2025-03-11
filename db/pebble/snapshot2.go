package pebble

import (
	"errors"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/cockroachdb/pebble"
)

var _ db.Snapshot = (*snapshot2)(nil)

type snapshot2 struct {
	snapshot *pebble.Snapshot
	listener db.EventListener
}

func NewSnapshot2(db *pebble.DB, listener db.EventListener) *snapshot2 {
	return &snapshot2{snapshot: db.NewSnapshot(), listener: listener}
}

func (s *snapshot2) Has(key []byte) (bool, error) {
	_, closer, err := s.snapshot.Get(key)
	if err != nil {
		return false, err
	}

	return true, closer.Close()
}

func (s *snapshot2) Get(key []byte) ([]byte, error) {
	start := time.Now()

	defer func() {
		s.listener.OnIO(false, time.Since(start))
	}()
	data, closer, err := s.snapshot.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, db.ErrKeyNotFound
		}
		return nil, err
	}

	return data, closer.Close()
}

func (s *snapshot2) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
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

func (s *snapshot2) Close() error {
	return s.snapshot.Close()
}
