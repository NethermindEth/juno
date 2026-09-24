package headstate

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/common"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

var (
	shouldRerun    = []byte{}
	shouldNotRerun = []byte(nil)
)

var _ migration.Migration = (*Migrator)(nil)

// Migrator consolidates the deprecated per-field contract layout into a
// single Contract record per address:
//
//	ContractClassHash[addr]
//	ContractNonce[addr]
//	ContractDeploymentHeight[addr]
//	          │
//	          ▼
//	Contract[addr] = { ClassHash, Nonce, DeployedHeight }
//
// StorageRoot is left zero — the running node lazily backfills it on the
// contract's first storage write.
//
// pendingContracts walks the buckets in lockstep; the records are batched and
// written here. Once every address has been migrated, the three deprecated
// buckets are wiped via DeleteRange.
//
// Re-run safe: an address already present in Contract is skipped, and the
// trailing wipe re-issues DeleteRange over the (possibly empty) ranges.
type Migrator struct{}

func (Migrator) Before([]byte) error {
	return nil
}

func (Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	_ *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	if err := migrateContracts(ctx, database, logger); err != nil {
		return shouldRerun, fmt.Errorf("migrating head state contracts: %w", err)
	}

	return shouldNotRerun, wipeDeprecatedBuckets(database)
}

// migrateContracts batches the merged source and writes at the target size.
func migrateContracts(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
) error {
	contracts, sourceErr := pendingContracts(database)
	counter := common.NewCounter(logger, common.TimeLogRate, "")
	batch := database.NewBatchWithSize(common.BatchByteSize)
	total, sinceLog := 0, 0

	for contract := range contracts {
		// Stop handing out contracts but keep the batch: it is written below, so
		// cancelling never throws away work already done.
		if ctx.Err() != nil {
			break
		}
		addr := (*felt.Felt)(&contract.addr)
		if err := state.WriteContract(
			batch, addr, contract.nonce, contract.classHash, contract.height,
		); err != nil {
			return fmt.Errorf("writing contract %s: %w", addr, err)
		}
		total++
		sinceLog++

		if batch.Size() >= common.TargetBatchByteSize {
			size := uint64(batch.Size())
			if err := batch.Write(); err != nil {
				return fmt.Errorf("writing batch: %w", err)
			}
			counter.Log(size, sinceLog, 0)
			sinceLog = 0
			batch = database.NewBatchWithSize(common.BatchByteSize)
		}
	}

	if err := sourceErr(); err != nil {
		return err
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("writing batch: %w", err)
	}

	logger.Info("Migrated head state contracts", zap.Int("contracts", total))
	return ctx.Err()
}

func wipeDeprecatedBuckets(database db.KeyValueStore) error {
	for _, bucket := range []db.Bucket{
		db.ContractClassHash,
		db.ContractNonce,
		db.ContractDeploymentHeight,
	} {
		start := bucket.Key()
		end := dbutils.UpperBound(start)
		if err := database.DeleteRange(start, end); err != nil {
			return err
		}
	}
	return nil
}
