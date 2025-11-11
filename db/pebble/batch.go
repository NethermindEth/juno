package pebble

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/cockroachdb/pebble"
)

var _ db.Batch = (*batch)(nil)

type batch struct {
	batch    *pebble.Batch
	db       *DB
	size     int // size of the batch in bytes
	listener db.EventListener
}

func NewBatch(dbBatch *pebble.Batch, db *DB, listener db.EventListener) *batch {
	return &batch{
		batch:    dbBatch,
		db:       db,
		listener: listener,
	}
}

// Delete : see db.Transaction.Delete
func (b *batch) Delete(key []byte) error {
	start := time.Now()
	defer func() { b.listener.OnIO(true, time.Since(start)) }()

	if err := b.batch.Delete(key, pebble.Sync); err != nil {
		return err
	}
	b.size += len(key)
	return nil
}

func (b *batch) DeleteRange(start, end []byte) error {
	return b.batch.DeleteRange(start, end, pebble.Sync)
}

//nolint:dupl
func (b *batch) Get(key []byte, cb func(value []byte) error) error {
	start := time.Now()
	defer func() { b.listener.OnIO(false, time.Since(start)) }()

	val, closer, err := b.batch.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return db.ErrKeyNotFound
		}
		return err
	}

	if err := cb(val); err != nil {
		return err
	}

	return closer.Close()
}

func (b *batch) Has(key []byte) (bool, error) {
	_, closer, err := b.batch.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, closer.Close()
}

func (b *batch) NewIterator(lowerBound []byte, withUpperBound bool) (db.Iterator, error) {
	var iter *pebble.Iterator
	var err error

	iterOpt := &pebble.IterOptions{LowerBound: lowerBound}
	if withUpperBound {
		iterOpt.UpperBound = dbutils.UpperBound(lowerBound)
	}

	iter, err = b.batch.NewIter(iterOpt)
	if err != nil {
		return nil, err
	}

	return &iterator{iter: iter}, nil
}

func (b *batch) Put(key, value []byte) error {
	if err := b.batch.Set(key, value, pebble.Sync); err != nil {
		return err
	}
	b.size += len(key) + len(value)
	return nil
}

func (b *batch) Size() int {
	return b.size
}

func (b *batch) Write() error {
	b.db.closeLock.RLock()
	defer b.db.closeLock.RUnlock()

	if b.db.closed {
		return pebble.ErrClosed
	}
	return b.batch.Commit(pebble.Sync)
}

func (b *batch) Reset() {
	b.batch.Reset()
	b.size = 0
}

type snapshotBatch struct {
	batch    *batch
	snapshot *snapshot
}

func NewSnapshotBatch(batch *batch, snapshot *snapshot) *snapshotBatch {
	return &snapshotBatch{
		batch:    batch,
		snapshot: snapshot,
	}
}

func (b *snapshotBatch) Put(key, value []byte) error {
	return b.batch.Put(key, value)
}

func (b *snapshotBatch) Delete(key []byte) error {
	return b.batch.Delete(key)
}

func (b *snapshotBatch) DeleteRange(start, end []byte) error {
	return b.batch.DeleteRange(start, end)
}

func (b *snapshotBatch) Size() int {
	return b.batch.Size()
}

func (b *snapshotBatch) Write() error {
	return b.batch.Write()
}

func (b *snapshotBatch) Reset() {
	b.batch.Reset()
}

func (b *snapshotBatch) Has(key []byte) (bool, error) {
	return b.snapshot.Has(key)
}

func (b *snapshotBatch) Get(key []byte, cb func(value []byte) error) error {
	return b.snapshot.Get(key, cb)
}

func (b *snapshotBatch) NewIterator(lowerBound []byte, withUpperBound bool) (db.Iterator, error) {
	return b.snapshot.NewIterator(lowerBound, withUpperBound)
}

type snapshotBatchWithBuffer struct {
	batch    *batch
	snapshot *snapshot
	buffer   map[string]*bufferEntry
	size     int
	ranges   []deleteRange
}

type bufferEntry struct {
	value []byte
	ok    bool
}

type deleteRange struct {
	start []byte
	end   []byte
}

func NewSnapshotBatchWithBuffer(batch *batch, snapshot *snapshot) *snapshotBatchWithBuffer {
	return &snapshotBatchWithBuffer{
		batch:    batch,
		snapshot: snapshot,
		buffer:   make(map[string]*bufferEntry),
	}
}

func (b *snapshotBatchWithBuffer) Put(key, value []byte) error {
	b.buffer[string(key)] = &bufferEntry{value: slices.Clone(value), ok: true}
	b.size += len(key) + len(value)
	return nil
}

func (b *snapshotBatchWithBuffer) Delete(key []byte) error {
	b.buffer[string(key)] = &bufferEntry{ok: false}
	b.size += len(key)
	return nil
}

func (b *snapshotBatchWithBuffer) DeleteRange(start, end []byte) error {
	b.ranges = append(b.ranges, deleteRange{
		start: slices.Clone(start),
		end:   slices.Clone(end),
	})
	return nil
}

func (b *snapshotBatchWithBuffer) Size() int {
	return b.size
}

func (b *snapshotBatchWithBuffer) Write() error {
	for _, r := range b.ranges {
		if err := b.batch.DeleteRange(r.start, r.end); err != nil {
			return err
		}
	}
	for k, entry := range b.buffer {
		key := []byte(k)
		if entry.ok {
			if err := b.batch.Put(key, entry.value); err != nil {
				return err
			}
		} else {
			if err := b.batch.Delete(key); err != nil {
				return err
			}
		}
	}
	if err := b.batch.Write(); err != nil {
		return err
	}
	b.buffer = make(map[string]*bufferEntry)
	b.ranges = b.ranges[:0]
	b.size = 0
	return nil
}

func (b *snapshotBatchWithBuffer) Reset() {
	b.buffer = make(map[string]*bufferEntry)
	b.ranges = b.ranges[:0]
	b.batch.Reset()
	b.size = 0
}

func (b *snapshotBatchWithBuffer) Get(key []byte, cb func([]byte) error) error {
	if entry, ok := b.buffer[string(key)]; ok {
		if !entry.ok {
			return db.ErrKeyNotFound
		}
		return cb(entry.value)
	}
	if inRange(b.ranges, key) {
		return db.ErrKeyNotFound
	}
	return b.snapshot.Get(key, cb)
}

func (b *snapshotBatchWithBuffer) Has(key []byte) (bool, error) {
	if entry, ok := b.buffer[string(key)]; ok {
		return entry.ok, nil
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

func (b *snapshotBatchWithBuffer) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
	return nil, fmt.Errorf("not implemented")
}
