package db

import (
	"bytes"
	"slices"
)

var _ SnapshotBatch = (*snapshotBatch)(nil)

type snapshotBatch struct {
	batch    Batch
	snapshot Snapshot
	writes   map[string][]byte
	deletes  map[string]struct{}
	size     int
	ranges   []deleteRange
}

type deleteRange struct {
	start []byte
	end   []byte
}

func NewSnapshotBatch(batch Batch, snapshot Snapshot) *snapshotBatch {
	return &snapshotBatch{
		batch:    batch,
		snapshot: snapshot,
		writes:   make(map[string][]byte),
		deletes:  make(map[string]struct{}),
	}
}

func (b *snapshotBatch) Put(key, value []byte) error {
	keyStr := string(key)
	delete(b.deletes, keyStr)
	b.writes[keyStr] = slices.Clone(value)
	b.size += len(key) + len(value)
	return nil
}

func (b *snapshotBatch) Delete(key []byte) error {
	keyStr := string(key)
	delete(b.writes, keyStr)
	b.deletes[keyStr] = struct{}{}
	b.size += len(key)
	return nil
}

func (b *snapshotBatch) DeleteRange(start, end []byte) error {
	b.ranges = append(b.ranges, deleteRange{
		start: slices.Clone(start),
		end:   slices.Clone(end),
	})
	return nil
}

func (b *snapshotBatch) Size() int {
	return b.size
}

func (b *snapshotBatch) Flush() error {
	for _, r := range b.ranges {
		if err := b.batch.DeleteRange(r.start, r.end); err != nil {
			return err
		}
	}
	for k, entry := range b.writes {
		key := []byte(k)
		if err := b.batch.Put(key, entry); err != nil {
			return err
		}
	}
	for k := range b.deletes {
		key := []byte(k)
		if err := b.batch.Delete(key); err != nil {
			return err
		}
	}

	b.writes = make(map[string][]byte)
	b.deletes = make(map[string]struct{})
	b.ranges = b.ranges[:0]
	b.size = 0
	return nil
}

func (b *snapshotBatch) Write() error {
	if err := b.Flush(); err != nil {
		return err
	}
	return b.batch.Write()
}

func (b *snapshotBatch) Reset() {
	b.writes = make(map[string][]byte)
	b.deletes = make(map[string]struct{})
	b.ranges = b.ranges[:0]
	b.batch.Reset()
	b.size = 0
}

func (b *snapshotBatch) Get(key []byte, cb func([]byte) error) error {
	if _, ok := b.deletes[string(key)]; ok {
		return ErrKeyNotFound
	}
	if entry, ok := b.writes[string(key)]; ok {
		return cb(entry)
	}
	if inRange(b.ranges, key) {
		return ErrKeyNotFound
	}
	return b.snapshot.Get(key, cb)
}

func (b *snapshotBatch) Has(key []byte) (bool, error) {
	if entry, ok := b.writes[string(key)]; ok {
		return entry != nil, nil
	}
	if _, ok := b.deletes[string(key)]; ok {
		return false, nil
	}
	if inRange(b.ranges, key) {
		return false, nil
	}
	return b.snapshot.Has(key)
}

func inRange(ranges []deleteRange, key []byte) bool {
	for _, r := range ranges {
		if bytes.Compare(key, r.start) >= 0 && bytes.Compare(key, r.end) < 0 {
			return true
		}
	}
	return false
}

func (b *snapshotBatch) NewIterator(prefix []byte, withUpperBound bool) (Iterator, error) {
	return b.snapshot.NewIterator(prefix, withUpperBound)
}
