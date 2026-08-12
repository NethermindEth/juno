package statedifflength

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
)

// ingestor reads blocks and fills per-worker batches. Each pipeline worker owns one
// entry in batches, so no synchronisation is needed between them.
type ingestor struct {
	database  db.KeyValueReader
	semaphore semaphore.ResourceSemaphore[db.Batch]
	progress  *progresslogger.BlockProgressTracker
	batches   []db.Batch
}

var _ pipeline.State[uint64, db.Batch] = (*ingestor)(nil)

func newIngestor(
	database db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	workers int,
	progress *progresslogger.BlockProgressTracker,
) *ingestor {
	batches := make([]db.Batch, workers)
	for i := range batches {
		batches[i] = batchSemaphore.GetBlocking()
	}
	return &ingestor{
		database:  database,
		semaphore: batchSemaphore,
		progress:  progress,
		batches:   batches,
	}
}

func (in *ingestor) Run(index int, blockNumber uint64, outputs chan<- db.Batch) error {
	batch := in.batches[index]

	if err := backfillBlock(in.database, batch, blockNumber); err != nil {
		return err
	}
	in.progress.IncrementCompletedBlocks(1)

	if batch.Size() >= targetBatchByteSize {
		outputs <- batch
		in.batches[index] = in.semaphore.GetBlocking()
	}
	return nil
}

func (in *ingestor) Done(index int, outputs chan<- db.Batch) error {
	outputs <- in.batches[index]
	return nil
}

// backfillBlock derives the state diff length from the block's state update and
// writes the block's commitments with it into batch. A missing commitments or
// state update record above the pruned prefix means the two buckets disagree, i.e.
// the database is corrupt, so the read error is returned rather than skipped.
func backfillBlock(r db.KeyValueReader, batch db.KeyValueWriter, blockNumber uint64) error {
	commitments, err := core.GetBlockCommitmentByBlockNum(r, blockNumber)
	if err != nil {
		return fmt.Errorf("getting commitments for block %d: %w", blockNumber, err)
	}

	stateUpdate, err := core.GetStateUpdateByBlockNum(r, blockNumber)
	if err != nil {
		return fmt.Errorf("getting state update for block %d: %w", blockNumber, err)
	}

	commitments.StateDiffLength = stateUpdate.StateDiff.Length()
	return core.WriteBlockCommitment(batch, blockNumber, commitments)
}
