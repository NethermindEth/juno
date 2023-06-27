package db

import (
	"errors"
)

var _ Transaction = (*memTransaction)(nil)

type memTransaction struct {
	storage map[string][]byte
}

func NewMemTransaction() Transaction {
	return &memTransaction{storage: make(map[string][]byte)}
}

func (t *memTransaction) NewIterator() (Iterator, error) {
	return nil, errors.New("not implemented")
}

func (t *memTransaction) Discard() error {
	t.storage = make(map[string][]byte)
	return nil
}

func (t *memTransaction) Commit() error {
	return nil
}

func (t *memTransaction) Set(key, val []byte) error {
	t.storage[string(key)] = append([]byte{}, val...)
	return nil
}

func (t *memTransaction) Delete(key []byte) error {
	delete(t.storage, string(key))
	return nil
}

func (t *memTransaction) Get(key []byte, cb func([]byte) error) error {
	value, found := t.storage[string(key)]
	if !found {
		return ErrKeyNotFound
	}
	return cb(value)
}

func (t *memTransaction) Impl() any {
	return t.storage
}
