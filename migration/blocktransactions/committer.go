package blocktransactions

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

type committer struct {
	counter        counter
	logger         utils.StructuredLogger
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
}

var _ pipeline.State[task, struct{}] = (*committer)(nil)

func newCommitter(
	logger utils.StructuredLogger,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
) *committer {
	return &committer{
		logger:         logger,
		counter:        newCounter(logger, timeLogRate),
		batchSemaphore: batchSemaphore,
	}
}

func (c *committer) Run(_ int, task task, _ chan<- struct{}) error {
	c.logger.Debug(
		"writing batch",
		zap.Int("txCount", task.totalTxCount),
		zap.Int("blockCount", task.totalBlockCount),
		zap.Int("batchSize", task.batch.Size()),
	)
	defer c.logger.Debug(
		"wrote batch",
		zap.Int("txCount", task.totalTxCount),
		zap.Int("blockCount", task.totalBlockCount),
		zap.Int("batchSize", task.batch.Size()),
	)

	byteSize := uint64(task.batch.Size())
	if err := task.batch.Write(); err != nil {
		return err
	}

	c.counter.log(byteSize, task.totalTxCount, task.totalBlockCount)
	c.batchSemaphore.Put()
	return nil
}

func (c *committer) Done(int, chan<- struct{}) error {
	return nil
}
