package historyprunner

import (
	"bytes"
	"encoding/binary"
	"sync/atomic"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
)

// stageTask is a key range [start, end) of one bucket.
type stageTask struct {
	bucket     db.Bucket
	start, end []byte
}

// stager stages one task range per run into its worker's batch, and counts
// what it scanned for the progress log.
type stager struct {
	reader  db.KeyValueReader
	sem     semaphore.ResourceSemaphore[db.Batch]
	layout  layout
	floor   uint64
	batches []db.Batch

	size, keys, entries atomic.Uint64
}

func newStager(
	reader db.KeyValueReader,
	sem semaphore.ResourceSemaphore[db.Batch],
	l layout,
	floor uint64,
	workers int,
) *stager {
	batches := make([]db.Batch, workers)
	for i := range batches {
		batches[i] = sem.GetBlocking()
	}
	return &stager{reader: reader, sem: sem, layout: l, floor: floor, batches: batches}
}

var _ pipeline.State[stageTask, db.Batch] = (*stager)(nil)

// Run stages task without watching ctx: a task handed out is always finished,
// which keeps the source's position an exact resume point.
func (s *stager) Run(index int, task stageTask, outputs chan<- db.Batch) error {
	it, err := s.reader.NewIterator(task.bucket.Key(), true)
	if err != nil {
		return err
	}
	defer it.Close()

	var size, keys, entries uint64
	defer func() {
		s.size.Add(size)
		s.keys.Add(keys)
		s.entries.Add(entries)
	}()

	batch := s.batches[index]
	ok := it.Seek(task.start)
	for ok {
		key := it.Key()
		if bytes.Compare(key, task.end) >= 0 {
			break
		}
		historyKey := key[:len(key)-blockNumberSuffixLen]
		if binary.BigEndian.Uint64(key[len(historyKey):]) < s.floor {
			ok = s.layout.seekStart(it, historyKey, s.floor)
		}
		for ; ok; ok = it.Next() {
			key := it.Key()
			if !bytes.HasPrefix(key, historyKey) {
				break
			}
			val, err := it.UncopiedValue()
			if err != nil {
				return err
			}
			if err := batch.Put(scratchKey(key), val); err != nil {
				return err
			}
			size += uint64(len(key) + len(val))
			entries++
		}
		keys++
		if batch.Size() >= targetBatchByteSize {
			outputs <- batch
			batch = s.sem.GetBlocking()
			s.batches[index] = batch
		}
	}
	return nil
}

func (s *stager) Done(index int, outputs chan<- db.Batch) error {
	outputs <- s.batches[index]
	return nil
}
