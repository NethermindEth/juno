package db

import (
	"slices"
)

// TODO: DO NOT USE THIS! This is meant to be a temporary replacement for buffered transaction.
// After state refactor, we can remove this.
type BufferBatch struct {
	updates map[string][]byte
	txn     IndexedBatch
}

func NewBufferBatch(txn IndexedBatch) *BufferBatch {
	return &BufferBatch{
		txn:     txn,
		updates: make(map[string][]byte),
	}
}

func (b *BufferBatch) Put(key, val []byte) error {
	b.updates[string(key)] = slices.Clone(val)
	return nil
}

func (b *BufferBatch) Delete(key []byte) error {
	b.updates[string(key)] = nil
	return nil
}

func (b *BufferBatch) Write() error {
	if err := b.Flush(); err != nil {
		return err
	}
	b.updates = nil
	return b.txn.Write()
}

func (b *BufferBatch) Get(key []byte, cb func(value []byte) error) error {
	if val, ok := b.updates[string(key)]; ok {
		if val == nil {
			return ErrKeyNotFound
		}
		return cb(val)
	}
	return b.txn.Get(key, cb)
}

func (b *BufferBatch) Flush() error {
	for key, val := range b.updates {
		if val == nil {
			if err := b.txn.Delete([]byte(key)); err != nil {
				return err
			}
		} else {
			if err := b.txn.Put([]byte(key), val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *BufferBatch) Close() error {
	return b.txn.Close()
}

func (b *BufferBatch) Has(key []byte) (bool, error) {
	panic("should not be called")
}

func (b *BufferBatch) NewIterator(prefix []byte, withUpperBound bool) (Iterator, error) {
	panic("should not be called")
}

func (b *BufferBatch) Size() int {
	panic("should not be called")
}

func (b *BufferBatch) DeleteRange(start, end []byte) error {
	panic("should not be called")
}
