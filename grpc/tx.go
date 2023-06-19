package grpc

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/db"
)

type tx struct {
	dbTx db.Transaction
	// index is cursorId for an iterator
	iterators []db.Iterator
}

func newTx(dbTx db.Transaction) *tx {
	return &tx{
		dbTx:      dbTx,
		iterators: make([]db.Iterator, 0),
	}
}

func (t *tx) newCursor() (uint32, error) {
	it, err := t.dbTx.NewIterator()
	if err != nil {
		return 0, err
	}
	t.iterators = append(t.iterators, it)

	cursorID := len(t.iterators) - 1
	return uint32(cursorID), nil
}

func (t *tx) iterator(cursorID uint32) (db.Iterator, error) {
	if int(cursorID) >= len(t.iterators) {
		return nil, fmt.Errorf("cursorID %d not found", cursorID)
	}

	return t.iterators[cursorID], nil
}

func (t *tx) cleanup() error {
	var err error
	for _, it := range t.iterators {
		err = errors.Join(err, it.Close())
	}

	return err
}
