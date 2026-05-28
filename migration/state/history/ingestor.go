package history

import (
	"context"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type baseIngestor struct {
	ctx            context.Context
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	database       db.KeyValueReader
	tasks          []task
}

// newBaseIngestor pre-allocates one batch per ingestor slot. The semaphore is
// created with capacity ingestorCount+1 immediately before this call, so the
// acquires cannot block — using GetBlocking keeps the constructor signature
// error-free.
func newBaseIngestor(
	ctx context.Context,
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) baseIngestor {
	tasks := make([]task, ingestorCount)
	for i := range tasks {
		tasks[i] = task{batch: sem.GetBlocking()}
	}
	return baseIngestor{
		ctx:            ctx,
		batchSemaphore: sem,
		database:       database,
		tasks:          tasks,
	}
}

// flush emits the current task downstream when its batch hits target size and
// acquires a fresh batch. The ctx-aware select on the channel send is the
// snappy cancellation point. The semaphore acquire uses GetBlocking — it is
// guaranteed to unblock within one committer iteration because the committer's
// deferred Put always runs.
func (b *baseIngestor) flush(t *task, outputs chan<- task) error {
	if t.batch.Size() < targetBatchByteSize {
		return nil
	}
	select {
	case <-b.ctx.Done():
		return b.ctx.Err()
	case outputs <- *t:
	}
	*t = task{batch: b.batchSemaphore.GetBlocking()}
	return nil
}

func (b *baseIngestor) Done(index int, outputs chan<- task) error {
	select {
	case <-b.ctx.Done():
		return b.ctx.Err()
	case outputs <- b.tasks[index]:
	}
	return nil
}
