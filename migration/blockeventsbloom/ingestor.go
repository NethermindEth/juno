package blockeventsbloom

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type task struct {
	batch      db.Batch
	blockCount int
}

type ingestor struct {
	database        db.KeyValueReader
	chainHeight     uint64
	batchSemaphore  semaphore.ResourceSemaphore[db.Batch]
	progressTracker *progresslogger.BlockProgressTracker
	tasks           []task
}

func newIngestor(
	database db.KeyValueReader,
	chainHeight uint64,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	progressTracker *progresslogger.BlockProgressTracker,
	ingestorCount int,
) *ingestor {
	tasks := make([]task, ingestorCount)
	for i := range tasks {
		tasks[i] = task{batch: batchSemaphore.GetBlocking()}
	}
	return &ingestor{
		database:        database,
		chainHeight:     chainHeight,
		batchSemaphore:  batchSemaphore,
		progressTracker: progressTracker,
		tasks:           tasks,
	}
}

var _ pipeline.State[uint64, task] = (*ingestor)(nil)

// Run migrates one batch of blocks ([startBlock, startBlock+batchSize),
// clamped to chainHeight), stripping each header's embedded event bloom. Headers
// decode into the current bloom-less core.Header (the decoder skips the legacy
// bloom) and are rewritten without it; re-running on a migrated header is a
// no-op. Only the retained range is scanned, so a missing header is a real
// error, not a pruned block.
func (c *ingestor) Run(index int, startBlock uint64, outputs chan<- task) error {
	endBlock := min(startBlock+batchSize-1, c.chainHeight)
	t := &c.tasks[index]

	for blockNumber := startBlock; blockNumber <= endBlock; blockNumber++ {
		header, err := core.GetBlockHeaderByNumber(c.database, blockNumber)
		if err != nil {
			return err
		}
		if err := core.WriteBlockHeaderByNumber(t.batch, header); err != nil {
			return err
		}
		t.blockCount++
	}

	c.progressTracker.IncrementCompletedBlocks(endBlock - startBlock + 1)

	if t.batch.Size() >= targetBatchByteSize {
		outputs <- *t
		*t = task{batch: c.batchSemaphore.GetBlocking()}
	}

	return nil
}

func (c *ingestor) Done(index int, outputs chan<- task) error {
	outputs <- c.tasks[index]
	return nil
}
