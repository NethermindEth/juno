package statedifflength

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
)

// task is a filled batch handed from a reader to the committer, with the number of
// blocks it covers for progress reporting.
type task struct {
	batch  db.Batch
	blocks int
}

// ingestor reads blocks and fills per-worker batches. Each pipeline worker owns one
// entry in batches, so no synchronisation is needed between them.
type ingestor struct {
	database  db.KeyValueReader
	semaphore semaphore.ResourceSemaphore[db.Batch]
	flushAt   int
	batches   []task
}

var _ pipeline.State[uint64, task] = (*ingestor)(nil)

func newIngestor(
	database db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	workers, flushAt int,
) *ingestor {
	batches := make([]task, workers)
	for i := range batches {
		batches[i] = task{batch: batchSemaphore.GetBlocking()}
	}
	return &ingestor{
		database:  database,
		semaphore: batchSemaphore,
		flushAt:   flushAt,
		batches:   batches,
	}
}

func (in *ingestor) Run(index int, blockNumber uint64, outputs chan<- task) error {
	cur := &in.batches[index]

	if err := backfillBlock(in.database, cur.batch, blockNumber); err != nil {
		return err
	}
	cur.blocks++

	if cur.batch.Size() >= in.flushAt {
		outputs <- *cur
		in.batches[index] = task{batch: in.semaphore.GetBlocking()}
	}
	return nil
}

func (in *ingestor) Done(index int, outputs chan<- task) error {
	outputs <- in.batches[index]
	return nil
}
