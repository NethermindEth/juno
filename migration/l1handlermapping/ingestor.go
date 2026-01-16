package l1handlermapping

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	progresslogger "github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type ingestor struct {
	database        db.KeyValueReader
	batchSemaphore  semaphore.ResourceSemaphore[db.Batch]
	batches         []db.Batch
	progressTracker *progresslogger.BlockNumberProgressTracker
}

func newIngestor(
	database db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	maxWorkers int,
	progressTracker *progresslogger.BlockNumberProgressTracker,
) *ingestor {
	batches := make([]db.Batch, maxWorkers)
	for i := range batches {
		batches[i] = batchSemaphore.GetBlocking()
	}
	return &ingestor{
		database:        database,
		batchSemaphore:  batchSemaphore,
		batches:         batches,
		progressTracker: progressTracker,
	}
}

var _ pipeline.State[uint64, db.Batch] = (*ingestor)(nil)

func (i *ingestor) Run(index int, blockNumber uint64, outputs chan<- db.Batch) error {
	txns, err := core.GetTxsByBlockNum(i.database, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get transactions for block %d: %w", blockNumber, err)
	}

	batch := i.batches[index]

	if err := core.WriteL1HandlerMsgHashes(batch, txns); err != nil {
		return fmt.Errorf("failed to write L1 handler mapping for block %d: %w", blockNumber, err)
	}

	if batch.Size() >= targetBatchByteSize {
		outputs <- batch
		i.batches[index] = i.batchSemaphore.GetBlocking()
	}

	i.progressTracker.IncrementCompletedBlocks(1)
	return nil
}

func (i *ingestor) Done(index int, outputs chan<- db.Batch) error {
	outputs <- i.batches[index]
	return nil
}
