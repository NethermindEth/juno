package headstate

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type committer struct {
	counter        counter
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
		counter:        newCounter(logger, timeLogRate),
		batchSemaphore: batchSemaphore,
	}
}

func (c *committer) Run(_ int, t task, _ chan<- struct{}) error {
	c.logger.Debug(
		"writing batch",
		zap.Int("addrCount", t.addrCount),
		zap.Int("batchSize", t.batch.Size()),
	)

	byteSize := uint64(t.batch.Size())
	if err := t.batch.Write(); err != nil {
		return err
	}

	c.counter.log(byteSize, t.addrCount)
	c.batchSemaphore.Put()
	return nil
}

func (c *committer) Done(int, chan<- struct{}) error {
	return nil
}
