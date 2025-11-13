package db

import (
	"sync"
)

// A wrapper around IndexedBatch that allows for thread-safe operations.
// Ideally, you shouldn't have to use this at all. If you need to write to batches concurrently,
// it's better to create a single batch for each goroutine and then merge them afterwards.
type SyncBatch struct {
	lock  sync.RWMutex
	batch SnapshotBatch
}

func NewSyncBatch(batch SnapshotBatch) *SyncBatch {
	return &SyncBatch{
		batch: batch,
	}
}

func (s *SyncBatch) Delete(key []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.batch.Delete(key)
}

func (s *SyncBatch) DeleteRange(start, end []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.batch.DeleteRange(start, end)
}

func (s *SyncBatch) Put(key, val []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.batch.Put(key, val)
}

func (s *SyncBatch) Write() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.batch.Write()
}

func (s *SyncBatch) Reset() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.batch.Reset()
}

func (s *SyncBatch) Get(key []byte, cb func(value []byte) error) error {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.batch.Get(key, cb)
}

func (s *SyncBatch) Has(key []byte) (bool, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.batch.Has(key)
}

func (s *SyncBatch) NewIterator(lowerBound []byte, withUpperBound bool) (Iterator, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.batch.NewIterator(lowerBound, withUpperBound)
}

func (s *SyncBatch) Size() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.batch.Size()
}

func (s *SyncBatch) Close() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.batch.Close()
}
