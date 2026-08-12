package statedifflength

import (
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
)

// committer writes the batches the readers fill and hands their semaphore slots
// back. It is the only pipeline stage that writes, so it runs single-threaded.
type committer struct {
	semaphore semaphore.ResourceSemaphore[db.Batch]
}

var _ pipeline.State[db.Batch, struct{}] = (*committer)(nil)

func newCommitter(batchSemaphore semaphore.ResourceSemaphore[db.Batch]) *committer {
	return &committer{semaphore: batchSemaphore}
}

func (c *committer) Run(_ int, batch db.Batch, _ chan<- struct{}) error {
	if err := batch.Write(); err != nil {
		return err
	}
	c.semaphore.Put()
	return nil
}

func (c *committer) Done(int, chan<- struct{}) error {
	return nil
}
