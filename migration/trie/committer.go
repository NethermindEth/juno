package trie

import (
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
)

type committer struct {
	batchSem semaphore.ResourceSemaphore[db.Batch]
	counter  counter
}

func newCommitter(
	logger log.StructuredLogger,
	batchSem semaphore.ResourceSemaphore[db.Batch],
) *committer {
	return &committer{
		batchSem: batchSem,
		counter:  newCounter(logger, timeLogRate),
	}
}

func (c *committer) Run(_ int, t task, _ chan<- struct{}) error {
	byteSize := uint64(t.batch.Size())
	if err := t.batch.Write(); err != nil {
		return fmt.Errorf("trie migration: batch write failed: %w", err)
	}
	c.counter.log(byteSize, t.tries)
	c.batchSem.Put()
	return nil
}

func (c *committer) Done(int, chan<- struct{}) error {
	return nil
}
