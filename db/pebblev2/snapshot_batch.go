package pebblev2

import (
	"bytes"
	"slices"

	"github.com/NethermindEth/juno/db"
)

type SnapshotBatch struct {
	batch    *batch
	snapshot *snapshot
	writes   map[string][]byte
	deletes  map[string]struct{}
	size     int
	ranges   []deleteRange
}

type deleteRange struct {
	start []byte
	end   []byte
}

func NewSnapshotBatch(batch *batch, snapshot *snapshot) *SnapshotBatch {
	return &SnapshotBatch{
		batch:    batch,
		snapshot: snapshot,
		writes:   make(map[string][]byte),
		deletes:  make(map[string]struct{}),
	}
}

func (b *SnapshotBatch) Put(key, value []byte) error {
	b.writes[string(key)] = slices.Clone(value)
	b.size += len(key) + len(value)
	return nil
}

func (b *SnapshotBatch) Delete(key []byte) error {
	b.deletes[string(key)] = struct{}{}
	b.size += len(key)
	return nil
}

func (b *SnapshotBatch) DeleteRange(start, end []byte) error {
	b.ranges = append(b.ranges, deleteRange{
		start: slices.Clone(start),
		end:   slices.Clone(end),
	})
	return nil
}

func (b *SnapshotBatch) Size() int {
	return b.size
}

func (b *SnapshotBatch) Write() error {
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

	if err := b.batch.Write(); err != nil {
		return err
	}
	b.writes = make(map[string][]byte)
	b.deletes = make(map[string]struct{})
	b.ranges = b.ranges[:0]
	b.size = 0
	return nil
}

func (b *SnapshotBatch) Reset() {
	b.writes = make(map[string][]byte)
	b.deletes = make(map[string]struct{})
	b.ranges = b.ranges[:0]
	b.batch.Reset()
	b.size = 0
}

func (b *SnapshotBatch) Get(key []byte, cb func([]byte) error) error {
	if entry, ok := b.writes[string(key)]; ok {
		return cb(entry)
	}
	if inRange(b.ranges, key) {
		return db.ErrKeyNotFound
	}
	return b.snapshot.Get(key, cb)
}

func (b *SnapshotBatch) Has(key []byte) (bool, error) {
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

func (b *SnapshotBatch) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
	return b.snapshot.NewIterator(prefix, withUpperBound)
}
