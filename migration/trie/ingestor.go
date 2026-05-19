package trie

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type task struct {
	batch db.Batch
	tries int
}

const dfsStackCap = 251

type ingestor struct {
	ctx            context.Context
	database       db.KeyValueReader
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	pool           *hashWorkerPool
	tasks          [IngestorCount]task
	dfsStacks      [IngestorCount][]dfsFrame
}

type FlushBatchFn func(db.Batch) (db.Batch, error)

func newIngestor(
	ctx context.Context,
	database db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	pool *hashWorkerPool,
) *ingestor {
	in := &ingestor{
		ctx:            ctx,
		database:       database,
		batchSemaphore: batchSemaphore,
		pool:           pool,
	}
	for i := range IngestorCount {
		in.tasks[i].batch = batchSemaphore.GetBlocking()
		in.dfsStacks[i] = make([]dfsFrame, 0, dfsStackCap)
	}
	return in
}

func (i *ingestor) Run(index int, desc TrieDesc, outputs chan<- task) error {
	done, err := hasDestRoot(i.database, desc.NewBucket, &desc.Owner)
	if err != nil {
		return fmt.Errorf("hasDestRoot(%v, %x): %w", desc.NewBucket, desc.Owner, err)
	}
	if done {
		return nil
	}

	t := &i.tasks[index]

	flush := FlushBatchFn(func(current db.Batch) (db.Batch, error) {
		if current.Size() < targetBatchByteSize {
			return current, nil
		}
		select {
		case <-i.ctx.Done():
			return current, i.ctx.Err()
		case outputs <- task{batch: current, tries: t.tries}:
		}
		t.tries = 0
		return i.batchSemaphore.GetBlocking(), nil
	})

	migrator := newDFSMigrator(desc.NodeCount >= SmallTrieThreshold, i.pool)
	t.batch, i.dfsStacks[index], err = migrator.Migrate(
		i.database,
		t.batch,
		desc,
		flush,
		i.dfsStacks[index],
	)
	if err != nil {
		return err
	}

	t.tries++
	t.batch, err = flush(t.batch)
	return err
}

func (i *ingestor) Done(index int, outputs chan<- task) error {
	select {
	case <-i.ctx.Done():
		return i.ctx.Err()
	case outputs <- i.tasks[index]:
	}
	return nil
}
