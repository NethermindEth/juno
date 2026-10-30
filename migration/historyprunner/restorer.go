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

// restorer rebuilds the hash lookups of one block per run: block hash to
// number, transaction hash to block and index, and L1 message hash to
// transaction hash.
type restorer struct {
	reader          db.KeyValueReader
	batchSemaphore  semaphore.ResourceSemaphore[db.Batch]
	batches         []db.Batch
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
		progressTracker: progressTracker,
	}
}

var _ pipeline.State[uint64, db.Batch] = (*restorer)(nil)

func (r *restorer) Run(index int, blockNumber uint64, outputs chan<- db.Batch) error {
	blockHash, err := core.GetBlockHeaderHashByNumber(r.reader, blockNumber)
	if err != nil {
		return fmt.Errorf("load block hash %d: %w", blockNumber, err)
	}

	batch := r.batches[index]
	if err := core.BlockHeaderNumbersByHashBucket.Put(batch, blockHash, &blockNumber); err != nil {
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
