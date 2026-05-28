package headstate

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils/log"
)

const (
	batchByteSize       = 128 * db.Megabyte
	targetBatchByteSize = 96 * db.Megabyte
	ingestorCount       = 4
	timeLogRate         = 5 * time.Second
)

var (
	shouldRerun    = []byte{}
	shouldNotRerun = []byte(nil)
)

var _ migration.Migration = (*Migrator)(nil)

// Migrator consolidates the deprecated per-field contract layout into a
// single Contract record per address, written via state.WriteContract:
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
// Each address discovered in the ContractClassHash bucket is processed by one
// of ingestorCount worker goroutines that read the three old fields into a
// shared db.Batch; a single committer drains batches to disk. Once every
// address has been migrated, the three deprecated buckets are wiped via
// DeleteRange.
//
// Re-run safe: an address whose Contract record already exists is skipped
// (via state.HasContract), and the trailing wipe re-issues DeleteRange over
// the (possibly already empty) ranges.
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
	addressesIter, sourceErr := pendingAddresses(database)
	res := migrateAddresses(ctx, database, logger, addressesIter)

	if err := errors.Join(sourceErr(), res.Err); err != nil {
		return shouldRerun, err
	}
	if !res.IsDone {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return shouldRerun, fmt.Errorf("migrating head state addresses: %w", ctxErr)
		}
		return shouldRerun, errors.New("migrating head state addresses: pipeline reported incomplete")
	}

	return shouldNotRerun, wipeDeprecatedBuckets(database)
}

func migrateAddresses(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
	addresses iter.Seq[felt.Address],
) pipeline.Result {
	batchSemaphore := semaphore.New(
		ingestorCount+1,
		func() db.Batch {
			return database.NewBatchWithSize(batchByteSize)
		},
	)

	source := pipeline.Source(addresses)

	ingestorPipeline := pipeline.New(
		source,
		ingestorCount,
		newIngestor(database, batchSemaphore),
	)

	committerPipeline := pipeline.New(
		ingestorPipeline,
		1,
		newCommitter(logger, batchSemaphore),
	)

	_, wait := committerPipeline.Run(ctx)
	return wait()
}

func pendingAddresses(r db.KeyValueReader) (iter.Seq[felt.Address], func() error) {
	var iterErr error
	seq := func(yield func(felt.Address) bool) {
		prefix := db.ContractClassHash.Key()
		it, err := r.NewIterator(prefix, true)
		if err != nil {
			iterErr = err
			return
		}
		defer it.Close()

		for valid := it.First(); valid; valid = it.Next() {
			key := it.Key()
			if len(key) != len(prefix)+felt.Bytes {
				iterErr = fmt.Errorf(
					"malformed ContractClassHash key: len %d, want %d",
					len(key),
					len(prefix)+felt.Bytes,
				)
				return
			}
			addr := felt.FromBytes[felt.Address](key[len(prefix):])
			if !yield(addr) {
				return
			}
		}
	}
	return seq, func() error { return iterErr }
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
