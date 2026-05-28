package history

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type committer struct {
	logger         log.StructuredLogger
	counter        counter
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
}

var _ pipeline.State[task, struct{}] = (*committer)(nil)

func newCommitter(
	logger log.StructuredLogger,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	phaseName string,
) *committer {
	return &committer{
		logger:         logger,
		counter:        newCounter(logger, timeLogRate, phaseName),
		batchSemaphore: batchSemaphore,
	}
}

func (c *committer) Run(_ int, t task, _ chan<- struct{}) error {
	defer c.batchSemaphore.Put()

	c.logger.Debug(
		"writing batch",
		zap.Int("completedAddrs", t.completedAddrs),
		zap.Int("entryCount", t.entryCount),
		zap.Int("batchSize", t.batch.Size()),
	)

	byteSize := uint64(t.batch.Size())
	if err := t.batch.Write(); err != nil {
		return err
	}

	c.counter.log(byteSize, t.completedAddrs, t.entryCount)
	return nil
}

func (c *committer) Done(int, chan<- struct{}) error {
	return nil
}
