package historyprunner

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type restorer struct {
	reader          db.KeyValueReader
	batchSemaphore  semaphore.ResourceSemaphore[db.Batch]
	batches         []db.Batch
	scratchPool     []copyScratch
	progressTracker *progresslogger.BlockProgressTracker
}

func newRestorer(
	reader db.KeyValueReader,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	maxWorkers int,
	progressTracker *progresslogger.BlockProgressTracker,
) *restorer {
	batches := make([]db.Batch, maxWorkers)
	for i := range batches {
		batches[i] = batchSemaphore.GetBlocking()
	}
	return &restorer{
		reader:          reader,
		batchSemaphore:  batchSemaphore,
		batches:         batches,
		scratchPool:     make([]copyScratch, maxWorkers),
		progressTracker: progressTracker,
	}
}

var _ pipeline.State[uint64, db.Batch] = (*restorer)(nil)

func (r *restorer) Run(index int, blockNumber uint64, outputs chan<- db.Batch) error {
	update, err := core.GetStateUpdateByBlockNum(r.reader, blockNumber)
	if err != nil {
		return fmt.Errorf("load state update for block %d: %w", blockNumber, err)
	}

	batch := r.batches[index]

	if err := copyStateHistory(
		r.reader,
		batch,
		&r.scratchPool[index],
		update.StateDiff,
		blockNumber,
		scratchToHistory,
	); err != nil {
		return err
	}

	err = core.WriteBlockHeaderNumberByHash(batch, update.BlockHash, blockNumber)
	if err != nil {
		return err
	}

	var idx uint64
	for tx, iterErr := range core.GetTransactionsByBlockNumberIter(r.reader, blockNumber) {
		if iterErr != nil {
			return fmt.Errorf("rebuild tx indices: block %d iter: %w", blockNumber, iterErr)
		}

		txHash := (*felt.TransactionHash)(tx.Hash())
		key := db.BlockNumIndexKey{Number: blockNumber, Index: idx}
		err := core.TransactionBlockNumbersAndIndicesByHashBucket.Put(
			batch, txHash, &key,
		)
		if err != nil {
			return fmt.Errorf(
				"rebuild tx indices: bucket 9 put block %d idx %d: %w",
				blockNumber, idx, err,
			)
		}

		if l1, ok := tx.(*core.L1HandlerTransaction); ok {
			err := core.WriteL1HandlerTxnHashByMsgHash(batch, l1.MessageHash(), l1.Hash())
			if err != nil {
				return fmt.Errorf(
					"rebuild tx indices: bucket 24 put block %d idx %d: %w",
					blockNumber,
					idx,
					err,
				)
			}
		}
		idx++
	}

	if batch.Size() >= targetBatchByteSize {
		outputs <- batch
		r.batches[index] = r.batchSemaphore.GetBlocking()
	}

	r.progressTracker.IncrementCompletedBlocks(1)
	return nil
}

func (r *restorer) Done(index int, outputs chan<- db.Batch) error {
	outputs <- r.batches[index]
	return nil
}
