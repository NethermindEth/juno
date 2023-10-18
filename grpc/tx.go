package grpc

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/NethermindEth/juno/db"
)

type tx struct {
	dbTx      db.Transaction
	itCounter atomic.Uint32
	// index is cursorId for an iterator
	iterators sync.Map
}

func newTx(dbTx db.Transaction) *tx {
	return &tx{
		dbTx: dbTx,
	}
}

func (t *tx) newCursor() (uint32, error) {
	it, err := t.dbTx.NewIterator()
	if err != nil {
		return 0, err
	}
	cursorID := t.itCounter.Add(1)
	t.iterators.Store(cursorID, it)
	return cursorID, nil
}

func (t *tx) closeCursor(cursorID uint32) error {
	it, err := t.iterator(cursorID)
	if err != nil {
		return err
	}
	t.iterators.Delete(cursorID)
	return it.Close()
}

func (t *tx) iterator(cursorID uint32) (db.Iterator, error) {
	it, found := t.iterators.Load(cursorID)
	if !found {
		return nil, fmt.Errorf("cursorID %d not found", cursorID)
	}
	return it.(db.Iterator), nil
}

func (t *tx) cleanup() error {
	var err error
	t.iterators.Range(func(key, value any) bool {
		it := value.(db.Iterator)
		err = errors.Join(err, it.Close())
		return true
	})
	return err
}
