package historyprunner

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// newStateHistoryBuckets hold history in the new-state layout: each entry
// records the value *after* its block, and a read at h takes the latest entry
// at or before h.
var newStateHistoryBuckets = []db.Bucket{
	db.ContractStorageHistory,
	db.ContractNonceHistory,
	db.ContractClassHashHistory,
}

// stageNewStateHistory copies to the scratch space every new-state history
// entry a read at or above cutoff can reach: the entries above cutoff, plus
// each key's latest entry at or below it, which answers reads until the key's
// next change. Unlike the deprecated layout, this set can't be derived from
// the kept blocks' state diffs, since a key untouched since cutoff still needs
// its latest entry, so the buckets are scanned in full.
//
// Re-run safe: scratch writes are idempotent and the source buckets are only
// wiped once staging completes. Returns false when ctx is cancelled.
func stageNewStateHistory(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
	cutoff uint64,
) (bool, error) {
	w := newBatchWriter(database)
	defer w.close()

	for _, bucket := range newStateHistoryBuckets {
		done, err := stageBucket(ctx, database, w, bucket, cutoff)
		if err != nil {
			return false, fmt.Errorf("staging %v: %w", bucket, err)
		}
		if !done {
			return false, nil
		}
	}
	if err := w.flush(); err != nil {
		return false, err
	}
	logger.Info("Staged new-state history", zap.Uint64("kept", w.written))
	return true, nil
}

func stageBucket(
	ctx context.Context,
	r db.KeyValueReader,
	w *batchWriter,
	bucket db.Bucket,
	cutoff uint64,
) (bool, error) {
	it, err := r.NewIterator(bucket.Key(), true)
	if err != nil {
		return false, err
	}
	defer it.Close()

	// The latest entry at or below cutoff for the key being scanned
	var pendingKey, pendingVal []byte
	flushPending := func() error {
		if pendingKey == nil {
			return nil
		}
		err := w.put(scratchKey(pendingKey), pendingVal)
		pendingKey, pendingVal = nil, nil
		return err
	}

	for ok := it.First(); ok; ok = it.Next() {
		if ctx.Err() != nil {
			return false, nil
		}

		key := it.Key()
		keyPrefix := key[:len(key)-blockNumberSuffixLen]
		if pendingKey != nil && !bytes.HasPrefix(pendingKey, keyPrefix) {
			if err := flushPending(); err != nil {
				return false, err
			}
		}

		val, err := it.Value()
		if err != nil {
			return false, err
		}
		if binary.BigEndian.Uint64(key[len(keyPrefix):]) <= cutoff {
			pendingKey, pendingVal = bytes.Clone(key), bytes.Clone(val)
			continue
		}
		if err := flushPending(); err != nil {
			return false, err
		}
		if err := w.put(scratchKey(key), val); err != nil {
			return false, err
		}
	}
	return true, flushPending()
}

// restoreNewStateHistory moves staged new-state history back into its buckets.
func restoreNewStateHistory(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
) (bool, error) {
	start := time.Now()
	w := newBatchWriter(database)
	defer w.close()

	for _, bucket := range newStateHistoryBuckets {
		done, err := restoreBucket(ctx, database, w, bucket)
		if err != nil || !done {
			return false, err
		}
	}
	if err := w.flush(); err != nil {
		return false, err
	}
	logger.Info("Restored new-state history", zap.Duration("elapsed", time.Since(start)))
	return true, nil
}

func restoreBucket(
	ctx context.Context,
	r db.KeyValueReader,
	w *batchWriter,
	bucket db.Bucket,
) (bool, error) {
	it, err := r.NewIterator([]byte{migrationScratchTag, byte(bucket)}, true)
	if err != nil {
		return false, err
	}
	defer it.Close()

	for ok := it.First(); ok; ok = it.Next() {
		if ctx.Err() != nil {
			return false, nil
		}
		val, err := it.Value()
		if err != nil {
			return false, err
		}
		if err := w.put(bytes.Clone(it.Key()[1:]), val); err != nil {
			return false, err
		}
	}
	return true, nil
}

func wipeNewStateHistoryBuckets(batch db.Batch) error {
	for _, bucket := range newStateHistoryBuckets {
		if err := wipeBucket(batch, byte(bucket)); err != nil {
			return fmt.Errorf("wiping %v: %w", bucket, err)
		}
	}
	return nil
}

// scratchKey prefixes a history key with the scratch tag: [Temporary][original key].
func scratchKey(key []byte) []byte {
	return append([]byte{migrationScratchTag}, key...)
}

// batchWriter puts entries into batches written at targetBatchByteSize.
type batchWriter struct {
	database db.KeyValueStore
	batch    db.Batch
	written  uint64
}

func newBatchWriter(database db.KeyValueStore) *batchWriter {
	return &batchWriter{
		database: database,
		batch:    database.NewBatchWithSize(int(batchByteSize)),
	}
}

func (w *batchWriter) put(key, val []byte) error {
	if err := w.batch.Put(key, val); err != nil {
		return err
	}
	w.written++
	if w.batch.Size() < targetBatchByteSize {
		return nil
	}
	if err := w.batch.Write(); err != nil {
		return err
	}
	w.batch = w.database.NewBatchWithSize(int(batchByteSize))
	return nil
}

func (w *batchWriter) flush() error {
	return w.batch.Write()
}

func (w *batchWriter) close() {
	_ = w.batch.Close()
}
