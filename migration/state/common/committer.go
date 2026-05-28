package common

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type Committer struct {
	logger         log.StructuredLogger
	counter        Counter
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	phaseName      string
}

var _ pipeline.State[Task, struct{}] = (*Committer)(nil)

func NewCommitter(
	logger log.StructuredLogger,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	phaseName string,
) *Committer {
	return &Committer{
		logger:         logger,
		counter:        NewCounter(logger, TimeLogRate, phaseName),
		batchSemaphore: batchSemaphore,
		phaseName:      phaseName,
	}
}

func (c *Committer) Run(_ int, t Task, _ chan<- struct{}) error {
	defer c.batchSemaphore.Put()

	fields := make([]zap.Field, 0, 4)
	if c.phaseName != "" {
		fields = append(fields, zap.String("phase", c.phaseName))
	}
	fields = append(fields,
		zap.Int("completedAddrs", t.CompletedAddrs),
		zap.Int("entryCount", t.EntryCount),
		zap.Int("batchSize", t.Batch.Size()),
	)
	c.logger.Debug("writing batch", fields...)

	byteSize := uint64(t.Batch.Size())
	if err := t.Batch.Write(); err != nil {
		return err
	}

	c.counter.Log(byteSize, t.CompletedAddrs, t.EntryCount)
	return nil
}

func (c *Committer) Done(int, chan<- struct{}) error {
	return nil
}
