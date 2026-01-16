package l1handlermapping

import (
	"fmt"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils"
)

type ingestor struct {
	database       db.KeyValueReader
	chainHeight    uint64
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	batches        []db.Batch
	logger         utils.SimpleLogger //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	startTime      time.Time
}

func newIngestor(
	logger utils.SimpleLogger, //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	database db.KeyValueReader,
	chainHeight uint64,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	maxWorkers int,
) *ingestor {
	batches := make([]db.Batch, maxWorkers)
	for i := range batches {
		batches[i] = batchSemaphore.GetBlocking()
	}
	return &ingestor{
		logger:         logger,
		database:       database,
		chainHeight:    chainHeight,
		batchSemaphore: batchSemaphore,
		batches:        batches,
		startTime:      time.Now(),
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

	if blockNumber > 0 && blockNumber%logRate == 0 {
		elapsed := time.Since(i.startTime)
		percentage := float64(blockNumber) / float64(i.chainHeight) * 100
		i.logger.Infow("L1 handler message hash migration progress",
			"percentage", fmt.Sprintf("%.2f%%", percentage),
			"elapsed", fmt.Sprintf("%.2fs", elapsed.Seconds()),
		)
	}

	return nil
}

func (i *ingestor) Done(index int, outputs chan<- db.Batch) error {
	outputs <- i.batches[index]
	return nil
}
