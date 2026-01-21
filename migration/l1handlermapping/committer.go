package l1handlermapping

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils"
)

type committer struct {
	logger         utils.SimpleLogger //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
}

var _ pipeline.State[db.Batch, struct{}] = (*committer)(nil)

func newCommitter(
	logger utils.SimpleLogger, //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
) *committer {
	return &committer{
		logger:         logger,
		batchSemaphore: batchSemaphore,
	}
}

func (c *committer) Run(_ int, batch db.Batch, _ chan<- struct{}) error {
	c.logger.Debugw(
		"writing batch",
		"batchSize", batch.Size(),
	)
	defer c.logger.Debugw(
		"wrote batch",
		"batchSize", batch.Size(),
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
