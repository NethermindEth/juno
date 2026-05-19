package trie

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
)

type task struct {
	batch db.Batch
	tries int
}

const dfsStackCap = 251

type ingestor struct {
	logger         log.StructuredLogger
	database       db.KeyValueReader
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	tasks          []task
	pool           *hashWorkerPool
	dfsStacks      [IngestorCount][]dfsFrame
}

type FlushBatchFn func(db.Batch) db.Batch

func newIngestor(
	ctx context.Context,
	database db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	logger log.StructuredLogger,
	pool *hashWorkerPool,
) (*ingestor, error) {
	tasks := make([]task, IngestorCount)
	for i := range tasks {
		tasks[i] = task{batch: batchSemaphore.GetBlocking()}
	}

	in := &ingestor{
		database:       database,
		batchSemaphore: batchSemaphore,
		logger:         logger,
		tasks:          tasks,
		pool:           pool,
	}
	for i := range in.dfsStacks {
		in.dfsStacks[i] = make([]dfsFrame, 0, dfsStackCap)
	}
	return in, nil
}

func (c *ingestor) Run(index int, desc TrieDesc, outputs chan<- task) error {
	done, err := hasDestRoot(c.database, desc.NewBucket, &desc.Owner)
	if err != nil {
		return fmt.Errorf("hasDestRoot(%v, %x): %w", desc.NewBucket, desc.Owner, err)
	}
	if done {
		return nil
	}

	t := &c.tasks[index]

	flush := FlushBatchFn(func(current db.Batch) db.Batch {
		if current.Size() < targetBatchByteSize {
			return current
		}
		outputs <- task{batch: current, tries: t.tries}
		t.tries = 0
		return c.batchSemaphore.GetBlocking()
	})

	migrator := newDFSMigrator(desc.NodeCount >= SmallTrieThreshold, c.pool)
	t.batch, c.dfsStacks[index], err = migrator.Migrate(
		c.database,
		t.batch,
		desc,
		flush,
		c.dfsStacks[index],
	)
	if err != nil {
		return err
	}

	t.tries++
	t.batch = flush(t.batch)
	return nil
}

func (c *ingestor) Done(index int, outputs chan<- task) error {
	outputs <- c.tasks[index]
	return nil
}
