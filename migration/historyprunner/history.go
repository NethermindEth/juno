package historyprunner

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/progresslogger"
)

// migrationScratchTag leads every scratch key: [Temporary][original key]. The
// original key keeps its bucket byte, so restoring needs no mapping.
var migrationScratchTag = byte(db.Temporary)

const blockNumberSuffixLen = 8

func scratchPrefix(bucket db.Bucket) []byte {
	return []byte{migrationScratchTag, byte(bucket)}
}

func scratchKey(key []byte) []byte {
	return append([]byte{migrationScratchTag}, key...)
}

// stageHistory copies to the scratch space every history entry that reads at
// or above floor reach, see [layout], starting at cursor. The source buckets
// are only wiped once staging completes, so a repeated run stages the same
// entries. On cancel it commits what it staged and returns the key to resume
// from.
func stageHistory(
	ctx context.Context,
	database db.KeyValueStore,
	l *layout,
	floor uint64,
	cursor []byte,
	counter *progresslogger.Counter,
) ([]byte, error) {
	buckets, err := l.bucketsFrom(cursor, 0)
	if err != nil {
		return nil, err
	}
	w := newBatchWriter(database)
	defer w.close()

	for _, bucket := range buckets {
		next, err := stageBucket(ctx, database, w, l, bucket, floor, cursor, counter)
		if err != nil {
			return nil, fmt.Errorf("staging %v: %w", bucket, err)
		}
		if next != nil {
			return next, w.flush()
		}
		cursor = nil
	}
	return nil, w.flush()
}

// stageBucket stages bucket from cursor on. It checks ctx between history
// keys, so a key's entries are staged all or none, and returns the first key
// left unstaged.
func stageBucket(
	ctx context.Context,
	r db.KeyValueReader,
	w *batchWriter,
	l *layout,
	bucket db.Bucket,
	floor uint64,
	cursor []byte,
	counter *progresslogger.Counter,
) ([]byte, error) {
	it, err := r.NewIterator(bucket.Key(), true)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	keep := func(key, val []byte) error {
		counter.Log(0, 0, 1)
		return w.put(scratchKey(key), val)
	}
	// The history key being scanned, and its latest entry below floor
	var historyKey, latestKey, latestVal []byte
	keepLatest := func() error {
		if latestKey == nil {
			return nil
		}
		err := keep(latestKey, latestVal)
		latestKey, latestVal = nil, nil
		return err
	}

	var ok bool
	if cursor != nil {
		ok = it.Seek(cursor)
	} else {
		ok = it.First()
	}
	for ; ok; ok = it.Next() {
		key := it.Key()
		entryKey := key[:len(key)-blockNumberSuffixLen]
		if !bytes.Equal(entryKey, historyKey) {
			if err := keepLatest(); err != nil {
				return nil, err
			}
			if ctx.Err() != nil {
				return entryKey, nil
			}
			historyKey = entryKey
		}

		val, err := it.Value()
		if err != nil {
			return nil, err
		}
		counter.Log(uint64(len(key)+len(val)), 1, 0)
		if binary.BigEndian.Uint64(key[len(entryKey):]) >= floor {
			if err := keepLatest(); err != nil {
				return nil, err
			}
			if err := keep(key, val); err != nil {
				return nil, err
			}
		} else if l.keepsLatestAtFloor {
			latestKey, latestVal = key, val
		}
	}
	return nil, keepLatest()
}

// restoreHistory copies the staged entries back into their buckets, starting
// at cursor. On cancel it commits what it copied and returns the scratch key
// to resume from.
func restoreHistory(
	ctx context.Context,
	database db.KeyValueStore,
	l *layout,
	cursor []byte,
	counter *progresslogger.Counter,
) ([]byte, error) {
	buckets, err := l.bucketsFrom(cursor, 1)
	if err != nil {
		return nil, err
	}
	w := newBatchWriter(database)
	defer w.close()

	for _, bucket := range buckets {
		next, err := restoreBucket(ctx, database, w, bucket, cursor, counter)
		if err != nil {
			return nil, fmt.Errorf("restoring %v: %w", bucket, err)
		}
		if next != nil {
			return next, w.flush()
		}
		cursor = nil
	}
	return nil, w.flush()
}

func restoreBucket(
	ctx context.Context,
	r db.KeyValueReader,
	w *batchWriter,
	bucket db.Bucket,
	cursor []byte,
	counter *progresslogger.Counter,
) ([]byte, error) {
	it, err := r.NewIterator(scratchPrefix(bucket), true)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var ok bool
	if cursor != nil {
		ok = it.Seek(cursor)
	} else {
		ok = it.First()
	}
	for ; ok; ok = it.Next() {
		if ctx.Err() != nil {
			return it.Key(), nil
		}
		key := it.Key()
		val, err := it.Value()
		if err != nil {
			return nil, err
		}
		if err := w.put(key[1:], val); err != nil {
			return nil, err
		}
		counter.Log(uint64(len(key)+len(val)), 1, 1)
	}
	return nil, nil
}

// batchWriter puts entries into batches written at targetBatchByteSize.
type batchWriter struct {
	database db.KeyValueStore
	batch    db.Batch
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
