package common

import (
	"context"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type BaseIngestor struct {
	Database       db.KeyValueReader
	Tasks          []Task
	ctx            context.Context
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
}

// NewBaseIngestor pre-allocates one batch per ingestor slot. The semaphore is
// created with capacity IngestorCount+1 immediately before this call, so the
// acquires cannot block — using GetBlocking keeps the constructor signature
// error-free.
func NewBaseIngestor(
	ctx context.Context,
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) BaseIngestor {
	tasks := make([]Task, IngestorCount)
	for i := range tasks {
		tasks[i] = Task{Batch: sem.GetBlocking()}
	}
	return BaseIngestor{
		Database:       database,
		Tasks:          tasks,
		ctx:            ctx,
		batchSemaphore: sem,
	}
}

// Flush emits the current task downstream when its batch hits target size and
// acquires a fresh batch. The ctx-aware select on the channel send is the
// snappy cancellation point. The semaphore acquire uses GetBlocking — it is
// guaranteed to unblock within one committer iteration because the committer's
// deferred Put always runs.
func (b *BaseIngestor) Flush(t *Task, outputs chan<- Task) error {
	if t.Batch.Size() < TargetBatchByteSize {
		return nil
	}
	select {
	case <-b.ctx.Done():
		return b.ctx.Err()
	case outputs <- *t:
	}
	*t = Task{Batch: b.batchSemaphore.GetBlocking()}
	return nil
}

func (b *BaseIngestor) Done(index int, outputs chan<- Task) error {
	select {
	case <-b.ctx.Done():
		return b.ctx.Err()
	case outputs <- b.Tasks[index]:
	}
	return nil
}
