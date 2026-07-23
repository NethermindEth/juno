package blockeventsbloom

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type committer struct {
	logger         log.StructuredLogger
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
}

var _ pipeline.State[task, struct{}] = (*committer)(nil)

func newCommitter(
	logger log.StructuredLogger,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
) *committer {
	return &committer{
		logger:         logger,
		batchSemaphore: batchSemaphore,
	}
}

func (c *committer) Run(_ int, task task, _ chan<- struct{}) error {
	c.logger.Debug(
		"writing batch",
		zap.Int("blockCount", task.blockCount),
		zap.Int("batchSize", task.batch.Size()),
	)

	if err := task.batch.Write(); err != nil {
		return err
	}

	c.batchSemaphore.Put()
	return nil
}

func (c *committer) Done(int, chan<- struct{}) error {
	return nil
}
