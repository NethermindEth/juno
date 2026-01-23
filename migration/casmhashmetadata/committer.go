package casmhashmetadata

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

type committer struct {
	logger         utils.StructuredLogger
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
}

var _ pipeline.State[db.Batch, struct{}] = (*committer)(nil)

func newCommitter(
	logger utils.StructuredLogger,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
) *committer {
	return &committer{
		logger:         logger,
		batchSemaphore: batchSemaphore,
	}
}

func (c *committer) Run(_ int, batch db.Batch, _ chan<- struct{}) error {
	c.logger.Debug(
		"writing batch",
		zap.Int("batch_size", batch.Size()),
	)
	defer c.logger.Debug(
		"wrote batch",
		zap.Int("batch_size", batch.Size()),
	)

	if err := batch.Write(); err != nil {
		return err
	}

	c.batchSemaphore.Put()
	return nil
}

func (c *committer) Done(int, chan<- struct{}) error {
	return nil
}
