package statedifflength

import (
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

const timeLogRate = 5 * time.Second

// committer writes the batches the readers produce. It is the only pipeline stage
// that writes, so it runs single-threaded and needs no locking for its counters.
type committer struct {
	logger    log.StructuredLogger
	semaphore semaphore.ResourceSemaphore[db.Batch]
	toBlock   uint64

	updated      uint64
	maxCommitted uint64
	lastLog      time.Time
}

var _ pipeline.State[task, struct{}] = (*committer)(nil)

func newCommitter(
	logger log.StructuredLogger,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	toBlock uint64,
) *committer {
	return &committer{
		logger:    logger,
		semaphore: batchSemaphore,
		toBlock:   toBlock,
		lastLog:   time.Now(),
	}
}

func (c *committer) Run(_ int, t task, _ chan<- struct{}) error {
	if err := t.batch.Write(); err != nil {
		return err
	}
	c.semaphore.Put()

	c.updated += uint64(t.blocks)
	if t.maxBlock > c.maxCommitted {
		c.maxCommitted = t.maxBlock
	}
	if time.Since(c.lastLog) >= timeLogRate {
		c.logger.Info("Backfilling state diff length",
			zap.Uint64("updated", c.updated),
			zap.Uint64("toBlock", c.toBlock),
		)
		c.lastLog = time.Now()
	}
	return nil
}

func (c *committer) Done(int, chan<- struct{}) error {
	c.logger.Info("Backfilled state diff length", zap.Uint64("updated", c.updated))
	return nil
}
