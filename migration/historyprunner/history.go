package historyprunner

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"iter"
	"runtime"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
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

// addrRanges is how many address ranges a bucket is split into, by the first
// two bytes of the address, which is below 2^251.
const addrRanges = 0x0800

// stageHistory copies to the scratch space every history entry that reads at
// or above floor reach, starting at resumeHistoryFrom. A source hands address
// ranges to workers; on cancel the ranges already
// handed out are finished and written, so it returns the start of the first
// range never handed out, the exact point to resume from.
func stageHistory(
	ctx context.Context,
	database db.KeyValueStore,
	l layout,
	floor uint64,
	resumeHistoryFrom []byte,
	logger log.StructuredLogger,
) ([]byte, error) {
	buckets, err := l.bucketsFrom(resumeHistoryFrom, 0)
	if err != nil {
		return nil, err
	}
	workers := runtime.GOMAXPROCS(0)
	sem := semaphore.New(workers+1, func() db.Batch {
		return database.NewBatchWithSize(int(batchByteSize))
	})

	s := newStager(database, sem, l, floor, workers)
	counter := newScanCounter(logger, "stage history", "historyKeys")
	stopLog := progresslogger.CallEveryInterval(ctx, timeLogRate, func() {
		counter.Log(s.size.Swap(0), int(s.keys.Swap(0)), int(s.entries.Swap(0)))
	})
	defer stopLog()

	var next []byte
	seq := stageTasks(buckets, resumeHistoryFrom, &next)
	stagers := pipeline.New(pipeline.Source(seq), workers, s)
	committers := pipeline.New(stagers, 1, newCommitter(logger, sem))
	_, wait := committers.Run(ctx)
	res := wait()
	if res.Err != nil {
		return nil, res.Err
	}
	if res.IsDone {
		return nil, nil
	}
	if next == nil {
		return nil, errors.New("stage stopped without a resume point")
	}
	return next, nil
}

// stageTasks yields the address ranges of buckets from resumeHistoryFrom on.
// When yield refuses a range it sets *next to its start.
func stageTasks(buckets []db.Bucket, resumeHistoryFrom []byte, next *[]byte) iter.Seq[stageTask] {
	return func(yield func(stageTask) bool) {
		for _, bucket := range buckets {
			from := 0
			if resumeHistoryFrom != nil {
				from = int(binary.BigEndian.Uint16(resumeHistoryFrom[1:]))
				resumeHistoryFrom = nil
			}
			for i := from; i < addrRanges; i++ {
				task := stageTask{
					bucket: bucket,
					start:  binary.BigEndian.AppendUint16(bucket.Key(), uint16(i)),
					end:    binary.BigEndian.AppendUint16(bucket.Key(), uint16(i+1)),
				}
				if i == addrRanges-1 {
					task.end = []byte{byte(bucket) + 1}
				}
				if !yield(task) {
					*next = task.start
					return
				}
			}
		}
	}
}

// restoreHistory copies the staged entries back into their buckets, starting
// at resumeHistoryFrom. On cancel it commits what it copied and returns the scratch key
// to resume from.
func restoreHistory(
	ctx context.Context,
	database db.KeyValueStore,
	l layout,
	resumeHistoryFrom []byte,
	counter *progresslogger.Counter,
) ([]byte, error) {
	buckets, err := l.bucketsFrom(resumeHistoryFrom, 1)
	if err != nil {
		return nil, err
	}
	w := newBatchWriter(database)
	defer w.close()

	for _, bucket := range buckets {
		next, err := restoreBucket(ctx, database, w, bucket, resumeHistoryFrom, counter)
		if err != nil {
			return nil, fmt.Errorf("restoring %v: %w", bucket, err)
		}
		if next != nil {
			return next, w.flush()
		}
		resumeHistoryFrom = nil
	}
	return nil, w.flush()
}

func restoreBucket(
	ctx context.Context,
	r db.KeyValueReader,
	w *batchWriter,
	bucket db.Bucket,
	resumeHistoryFrom []byte,
	counter *progresslogger.Counter,
) ([]byte, error) {
	it, err := r.NewIterator(scratchPrefix(bucket), true)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var ok bool
	if resumeHistoryFrom != nil {
		ok = it.Seek(resumeHistoryFrom)
	} else {
		ok = it.First()
	}
	for ; ok; ok = it.Next() {
		if ctx.Err() != nil {
			return it.Key(), nil
		}
		key := it.Key()
		val, err := it.UncopiedValue()
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
