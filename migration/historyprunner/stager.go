package historyprunner

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type stager struct {
	reader          db.KeyValueReader
	batchSemaphore  semaphore.ResourceSemaphore[db.Batch]
	batches         []db.Batch
	scratchPool     []copyScratch
	progressTracker *progresslogger.BlockProgressTracker
}

func newStager(
	reader db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	maxWorkers int,
	progressTracker *progresslogger.BlockProgressTracker,
) *stager {
	batches := make([]db.Batch, maxWorkers)
	for i := range batches {
		batches[i] = batchSemaphore.GetBlocking()
	}
	return &stager{
		reader:          reader,
		batchSemaphore:  batchSemaphore,
		batches:         batches,
		scratchPool:     make([]copyScratch, maxWorkers),
		progressTracker: progressTracker,
	}
}

var _ pipeline.State[uint64, db.Batch] = (*stager)(nil)

func (s *stager) Run(index int, blockNumber uint64, outputs chan<- db.Batch) error {
	update, err := core.GetStateUpdateByBlockNum(s.reader, blockNumber)
	if err != nil {
		return fmt.Errorf("load state update for block %d: %w", blockNumber, err)
	}

	batch := s.batches[index]

	if err := copyStateHistory(
		s.reader,
		batch,
		&s.scratchPool[index],
		update.StateDiff,
		blockNumber,
		historyToScratch,
	); err != nil {
		return err
	}

	if batch.Size() >= targetBatchByteSize {
		outputs <- batch
		s.batches[index] = s.batchSemaphore.GetBlocking()
	}

	s.progressTracker.IncrementCompletedBlocks(1)
	return nil
}

func (s *stager) Done(index int, outputs chan<- db.Batch) error {
	outputs <- s.batches[index]
	return nil
}
