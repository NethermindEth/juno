package db

import (
	"slices"
)

// ReaderBatch presents a read-only reader as an IndexedBatch for code that
// demands one just to read. Writes land in an in-memory overlay readable back
// through Get and Has, and are dropped with the batch.
// Temporary until the state refactor removes IndexedBatch from read paths.
type ReaderBatch struct {
	reader  KeyValueReader
	updates map[string][]byte
}

func NewReaderBatch(reader KeyValueReader) *ReaderBatch {
	return &ReaderBatch{reader: reader}
}

func (b *ReaderBatch) Get(key []byte, cb func(value []byte) error) error {
	if value, ok := b.updates[string(key)]; ok {
		if value == nil {
			return ErrKeyNotFound
		}
		return cb(value)
	}
	return b.reader.Get(key, cb)
}

func (b *ReaderBatch) Has(key []byte) (bool, error) {
	if value, ok := b.updates[string(key)]; ok {
		return value != nil, nil
	}
	return b.reader.Has(key)
}

// NewIterator iterates the wrapped reader only: read paths never iterate keys
// they wrote, so the overlay is not merged in.
func (b *ReaderBatch) NewIterator(prefix []byte, withUpperBound bool) (Iterator, error) {
	return b.reader.NewIterator(prefix, withUpperBound)
}

func (b *ReaderBatch) Put(key, value []byte) error {
	if b.updates == nil {
		b.updates = make(map[string][]byte)
	}
	b.updates[string(key)] = slices.Clone(value)
	return nil
}

func (b *ReaderBatch) Delete(key []byte) error {
	if b.updates == nil {
		b.updates = make(map[string][]byte)
	}
	b.updates[string(key)] = nil
	return nil
}

// Close does nothing: the wrapped reader's lifetime is owned by the caller.
func (b *ReaderBatch) Close() error {
	return nil
}

func (b *ReaderBatch) Write() error {
	panic("should not be called")
}

func (b *ReaderBatch) DeleteRange(start, end []byte) error {
	panic("should not be called")
}

func (b *ReaderBatch) Size() int {
	panic("should not be called")
}
