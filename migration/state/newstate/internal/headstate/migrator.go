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
	completed := 0

	write := func() error {
		size := uint64(batch.Size())
		if err := batch.Write(); err != nil {
			return err
		}
		counter.Log(size, completed, 0)
		completed = 0
		batch = database.NewBatchWithSize(common.BatchByteSize)
		return nil
	}

	for pc := range contracts {
		if err := ctx.Err(); err != nil {
			return err
		}
		addr := (*felt.Felt)(&pc.addr)
		if err := state.WriteContract(batch, addr, pc.nonce, pc.classHash, pc.height); err != nil {
			return fmt.Errorf("WriteContract(%s): %w", &pc.addr, err)
		}
		completed++

		if batch.Size() >= common.TargetBatchByteSize {
			if err := write(); err != nil {
				return err
			}
		}
	}

	if err := sourceErr(); err != nil {
		return err
	}
	return write()
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
