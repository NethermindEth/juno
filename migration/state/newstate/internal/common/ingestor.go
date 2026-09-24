package common

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type BaseIngestor struct {
	Database       db.KeyValueReader
	Tasks          []Task
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
}

// NewBaseIngestor pre-allocates one batch per ingestor slot. The semaphore is
// created with capacity IngestorCount+1 immediately before this call, so the
// acquires cannot block — using GetBlocking keeps the constructor signature
// error-free.
func NewBaseIngestor(
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
		batchSemaphore: sem,
	}
}

// Flush emits the current task downstream when its batch hits target size and
// acquires a fresh batch.
//
// It deliberately does not watch for cancellation. When the context ends the
// source stops handing out addresses, but every address already handed out is
// finished and committed; that is what makes the source's position an exact
// resume point. The semaphore acquire uses GetBlocking — it is guaranteed to
// unblock within one committer iteration because the committer's deferred Put
// always runs.
func (b *BaseIngestor) Flush(t *Task, outputs chan<- Task) error {
	if t.Batch.Size() < TargetBatchByteSize {
		return nil
	}
	outputs <- *t
	*t = Task{Batch: b.batchSemaphore.GetBlocking()}
	return nil
}

// Done hands the worker's final, partial task to the committer.
func (b *BaseIngestor) Done(index int, outputs chan<- Task) error {
	outputs <- b.Tasks[index]
	return nil
}
