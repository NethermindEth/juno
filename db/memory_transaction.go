package db

import (
	"errors"
	"sync"
)

var _ Transaction = (*memTransaction)(nil)

type memTransaction struct {
	storage map[string][]byte
	rwlock  *sync.RWMutex
}

func NewMemTransaction() Transaction {
	return &memTransaction{
		storage: make(map[string][]byte),
		rwlock:  &sync.RWMutex{},
	}
}

func (t *memTransaction) NewIterator() (Iterator, error) {
	return nil, errors.New("not implemented")
}

func (t *memTransaction) Discard() error {
	t.rwlock.Lock()
	defer t.rwlock.Unlock()
	t.storage = make(map[string][]byte)
	return nil
}

func (t *memTransaction) Commit() error {
	t.rwlock.Lock()
	defer t.rwlock.Unlock()
	return nil
}

func (t *memTransaction) Set(key, val []byte) error {
	t.rwlock.Lock()
	defer t.rwlock.Unlock()
	t.storage[string(key)] = append([]byte{}, val...)
	return nil
}

func (t *memTransaction) Delete(key []byte) error {
	t.rwlock.Lock()
	defer t.rwlock.Unlock()
	delete(t.storage, string(key))
	return nil
}

func (t *memTransaction) Get(key []byte, cb func([]byte) error) error {
	t.rwlock.RLock()
	defer t.rwlock.RUnlock()
	value, found := t.storage[string(key)]
	if !found {
		return ErrKeyNotFound
	}
	return cb(value)
}

func (t *memTransaction) Impl() any {
	return t.storage
}
