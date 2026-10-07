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
	w := newScratchWriter(database)
	defer w.close()

	for _, bucket := range newStateHistoryBuckets {
		done, err := stageBucket(ctx, database, w, bucket, cutoff)
		if err != nil {
			return false, fmt.Errorf("staging %v: %w", bucket, err)
		}
		if !done {
			return false, w.flush()
		}
		logger.Info("Staged new-state history",
			zap.Stringer("bucket", bucket),
			zap.Uint64("kept", w.written),
		)
		w.written = 0
	}
	return true, w.flush()
}

func stageBucket(
	ctx context.Context,
	r db.KeyValueReader,
	w *scratchWriter,
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
		err := w.put(pendingKey, pendingVal)
		pendingKey, pendingVal = nil, nil
		return err
	}

	for ok := it.First(); ok; ok = it.Next() {
		if ctx.Err() != nil {
			return false, nil
		}

		key := it.Key()
		if len(key) <= blockNumberSuffixLen {
			return false, fmt.Errorf("history key %x too short", key)
		}
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
		if err := w.put(key, val); err != nil {
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
	batch := database.NewBatchWithSize(int(batchByteSize))
	defer func() { _ = batch.Close() }()

	for _, bucket := range newStateHistoryBuckets {
		it, err := database.NewIterator([]byte{migrationScratchTag, byte(bucket)}, true)
		if err != nil {
			return false, err
		}
		for ok := it.First(); ok; ok = it.Next() {
			if ctx.Err() != nil {
				return false, it.Close()
			}
			val, err := it.Value()
			if err != nil {
				return false, joinClose(err, it)
			}
			if err := batch.Put(bytes.Clone(it.Key()[1:]), val); err != nil {
				return false, joinClose(err, it)
			}
			if batch.Size() >= targetBatchByteSize {
				if err := batch.Write(); err != nil {
					return false, joinClose(err, it)
				}
				batch = database.NewBatchWithSize(int(batchByteSize))
			}
		}
		if err := it.Close(); err != nil {
			return false, err
		}
	}
	if err := batch.Write(); err != nil {
		return false, err
	}
	logger.Info("Restored new-state history", zap.Duration("elapsed", time.Since(start)))
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

// scratchWriter writes scratch entries, keyed [Temporary][original key], in
// batches flushed at targetBatchByteSize.
type scratchWriter struct {
	database db.KeyValueStore
	batch    db.Batch
	written  uint64
}

func newScratchWriter(database db.KeyValueStore) *scratchWriter {
	return &scratchWriter{
		database: database,
		batch:    database.NewBatchWithSize(int(batchByteSize)),
	}
}

func (w *scratchWriter) put(key, val []byte) error {
	scratchKey := make([]byte, 1+len(key))
	scratchKey[0] = migrationScratchTag
	copy(scratchKey[1:], key)
	if err := w.batch.Put(scratchKey, val); err != nil {
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

func (w *scratchWriter) flush() error {
	return w.batch.Write()
}

func (w *scratchWriter) close() {
	_ = w.batch.Close()
}

func joinClose(err error, it db.Iterator) error {
	if closeErr := it.Close(); closeErr != nil {
		return fmt.Errorf("%w; closing iterator: %w", err, closeErr)
	}
	return err
}
