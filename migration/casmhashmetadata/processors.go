package casmhashmetadata

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
)

// pre0141Processor processes pre-0.14.1 blocks.
type pre0141Processor struct {
	database        db.KeyValueStore
	batch           db.Batch
	blocksProcessed *atomic.Uint64
	mu              sync.Mutex
}

var _ pipeline.State[uint64, struct{}] = (*pre0141Processor)(nil)

func (p *pre0141Processor) Run(_ int, blockNumber uint64, _ chan<- struct{}) error {
	stateUpdate, err := core.GetStateUpdateByBlockNum(p.database, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get state update for block %d: %w", blockNumber, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

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
			p.batch,
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

	p.blocksProcessed.Add(1)
	return nil
}

func (p *pre0141Processor) Done(_ int, _ chan<- struct{}) error {
	return nil
}

// post0141Processor processes blocks from 0.14.1 onwards.
type post0141Processor struct {
	database        db.KeyValueStore
	batch           db.Batch
	blocksProcessed *atomic.Uint64
	mu              sync.Mutex
}

var _ pipeline.State[uint64, struct{}] = (*post0141Processor)(nil)

func (p *post0141Processor) Run(_ int, blockNumber uint64, _ chan<- struct{}) error {
	stateUpdate, err := core.GetStateUpdateByBlockNum(p.database, blockNumber)
	if err != nil {
		return fmt.Errorf("failed to get state update for block %d: %w", blockNumber, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Process DeclaredV1Classes (V2 declarations in 0.14.1 and onwards)
	for sierraClassHash, casmHashV2 := range stateUpdate.StateDiff.DeclaredV1Classes {
		metadata := core.NewCasmHashMetadataDeclaredV2(
			blockNumber,
			(*felt.CasmClassHash)(casmHashV2),
		)

		err = core.WriteClassCasmHashMetadata(
			p.batch,
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

		err = core.WriteClassCasmHashMetadata(p.batch, &sierraClassHash, &metadata)
		if err != nil {
			return fmt.Errorf(
				"failed to write class CASM hash metadata for migrated class %s at block %d: %w",
				sierraClassHash.String(),
				blockNumber,
				err,
			)
		}
	}

	p.blocksProcessed.Add(1)
	return nil
}

func (p *post0141Processor) Done(_ int, _ chan<- struct{}) error {
	return nil
}
