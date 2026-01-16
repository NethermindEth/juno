package casmhashmetadata

import (
	"fmt"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils"
)

// pre0141Ingestor processes pre-0.14.1 blocks.
type pre0141Ingestor struct {
	database       db.KeyValueReader
	chainHeight    uint64
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	batches        []db.Batch
	logger         utils.SimpleLogger //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	startTime      time.Time
}

var _ pipeline.State[uint64, db.Batch] = (*pre0141Ingestor)(nil)

func newPre0141Ingestor(
	logger utils.SimpleLogger, //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	database db.KeyValueReader,
	chainHeight uint64,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	maxWorkers int,
	startTime time.Time,
) *pre0141Ingestor {
	batches := make([]db.Batch, maxWorkers)
	for i := range batches {
		batches[i] = batchSemaphore.GetBlocking()
	}
	return &pre0141Ingestor{
		logger:         logger,
		database:       database,
		chainHeight:    chainHeight,
		batchSemaphore: batchSemaphore,
		batches:        batches,
		startTime:      startTime,
	}
}

func (p *pre0141Ingestor) Run(index int, blockNumber uint64, outputs chan<- db.Batch) error {
	stateUpdate, err := core.GetStateUpdateByBlockNum(p.database, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get state update for block %d: %w", blockNumber, err)
	}
	batch := p.batches[index]
	// Process DeclaredV1Classes: create V1 records with both V1 and V2 hashes
	for sierraClassHash, casmHashV1 := range stateUpdate.StateDiff.DeclaredV1Classes {
		casmHashV2, err := deprecatedCasmHashV2Bucket.Get(
			p.database,
			(*felt.SierraClassHash)(&sierraClassHash),
		)
		if err != nil {
			return fmt.Errorf("failed to get V2 hash for declared class %s at block %d: %w",
				sierraClassHash.String(),
				blockNumber,
				err,
			)
		}

		metadata := core.NewCasmHashMetadataDeclaredV1(
			blockNumber,
			(*felt.CasmClassHash)(casmHashV1),
			&casmHashV2,
		)

		err = core.WriteClassCasmHashMetadata(
			batch,
			(*felt.SierraClassHash)(&sierraClassHash),
			&metadata,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to write class CASM hash metadata for declared class %s at block %d: %w",
				sierraClassHash.String(),
				blockNumber,
				err,
			)
		}
	}

	if batch.Size() >= targetBatchByteSize {
		outputs <- batch
		p.batches[index] = p.batchSemaphore.GetBlocking()
	}

	if blockNumber > 0 && blockNumber%logRate == 0 {
		elapsed := time.Since(p.startTime)
		percentage := float64(blockNumber) / float64(p.chainHeight) * 100
		p.logger.Infow("L1 handler message hash migration progress",
			"percentage", fmt.Sprintf("%.2f%%", percentage),
			"elapsed", fmt.Sprintf("%.2fs", elapsed.Seconds()),
		)
	}
	return nil
}

func (p *pre0141Ingestor) Done(index int, outputs chan<- db.Batch) error {
	outputs <- p.batches[index]
	return nil
}

// post0141Ingestor processes blocks from 0.14.1 onwards.
type post0141Ingestor struct {
	database       db.KeyValueReader
	chainHeight    uint64
	batchSemaphore semaphore.ResourceSemaphore[db.Batch]
	batches        []db.Batch
	logger         utils.SimpleLogger //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	startTime      time.Time
}

var _ pipeline.State[uint64, db.Batch] = (*post0141Ingestor)(nil)

func newPost0141Ingestor(
	logger utils.SimpleLogger, //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	database db.KeyValueReader,
	chainHeight uint64,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	maxWorkers int,
	startTime time.Time,
) *post0141Ingestor {
	batches := make([]db.Batch, maxWorkers)
	for i := range batches {
		batches[i] = batchSemaphore.GetBlocking()
	}
	return &post0141Ingestor{
		logger:         logger,
		database:       database,
		chainHeight:    chainHeight,
		batchSemaphore: batchSemaphore,
		batches:        batches,
		startTime:      startTime,
	}
}

func (p *post0141Ingestor) Run(index int, blockNumber uint64, outputs chan<- db.Batch) error {
	stateUpdate, err := core.GetStateUpdateByBlockNum(p.database, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get state update for block %d: %w", blockNumber, err)
	}
	batch := p.batches[index]
	// Process DeclaredV1Classes (V2 declarations in 0.14.1 and onwards)
	for sierraClassHash, casmHashV2 := range stateUpdate.StateDiff.DeclaredV1Classes {
		metadata := core.NewCasmHashMetadataDeclaredV2(
			blockNumber,
			(*felt.CasmClassHash)(casmHashV2),
		)

		err = core.WriteClassCasmHashMetadata(
			batch,
			(*felt.SierraClassHash)(&sierraClassHash),
			&metadata,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to write class CASM hash metadata for declared class %s at block %d: %w",
				sierraClassHash.String(),
				blockNumber,
				err,
			)
		}
	}

	// Process MigratedClasses
	for sierraClassHash := range stateUpdate.StateDiff.MigratedClasses {
		// Get existing record (must exist from phase 1, which was already flushed)
		metadata, err := core.GetClassCasmHashMetadata(p.database, &sierraClassHash)
		if err != nil {
			return fmt.Errorf(
				"failed to get existing metadata for migrated class %s at block %d: %w",
				sierraClassHash.String(),
				blockNumber,
				err,
			)
		}

		if err := metadata.Migrate(blockNumber); err != nil {
			return fmt.Errorf(
				"failed to migrate class %s at block %d: %w",
				sierraClassHash.String(),
				blockNumber,
				err,
			)
		}

		err = core.WriteClassCasmHashMetadata(batch, &sierraClassHash, &metadata)
		if err != nil {
			return fmt.Errorf(
				"failed to write class CASM hash metadata for migrated class %s at block %d: %w",
				sierraClassHash.String(),
				blockNumber,
				err,
			)
		}
	}

	if batch.Size() >= targetBatchByteSize {
		outputs <- batch
		p.batches[index] = p.batchSemaphore.GetBlocking()
	}

	if blockNumber > 0 && blockNumber%logRate == 0 {
		elapsed := time.Since(p.startTime)
		percentage := float64(blockNumber) / float64(p.chainHeight) * 100
		p.logger.Infow("Casm hash metadata migration progress",
			"percentage", fmt.Sprintf("%.2f%%", percentage),
			"elapsed", fmt.Sprintf("%.2fs", elapsed.Seconds()),
		)
	}
	return nil
}

func (p *post0141Ingestor) Done(index int, outputs chan<- db.Batch) error {
	outputs <- p.batches[index]
	return nil
}
