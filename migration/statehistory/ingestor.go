package statehistory

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type FlushBatchFn func(t *task)

type ingestor struct {
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	database       db.KeyValueReader
	tasks          []task
	transform      func(db.KeyValueReader, *task, felt.Address, FlushBatchFn) error
}

var _ pipeline.State[felt.Address, task] = (*ingestor)(nil)

func newIngestor(
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
	transform func(db.KeyValueReader, *task, felt.Address, FlushBatchFn) error,
) *ingestor {
	tasks := make([]task, ingestorCount)
	for i := range tasks {
		tasks[i] = task{batch: sem.GetBlocking()}
	}
	return &ingestor{batchSemaphore: sem, database: database, tasks: tasks, transform: transform}
}

func (p *ingestor) Run(index int, addr felt.Address, outputs chan<- task) error {
	t := &p.tasks[index]
	flush := func(t *task) {
		if t.batch.Size() < targetBatchByteSize {
			return
		}
		outputs <- *t
		*t = task{batch: p.batchSemaphore.GetBlocking()}
	}

	if err := p.transform(p.database, t, addr, flush); err != nil {
		return err
	}

	if t.batch.Size() >= targetBatchByteSize {
		outputs <- *t
		*t = task{batch: p.batchSemaphore.GetBlocking()}
	}
	return nil
}

func (p *ingestor) Done(index int, outputs chan<- task) error {
	outputs <- p.tasks[index]
	return nil
}
