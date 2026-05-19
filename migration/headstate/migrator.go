package headstate

import (
	"context"
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

type task struct {
	batch     db.Batch
	addrCount int
}

var (
	shouldRerun    = []byte{}
	shouldNotRerun = []byte(nil)
)

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

var _ migration.Migration = (*Migrator)(nil)

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
	hasPending, err := hasPendingAddresses(database)
	if err != nil {
		return shouldRerun, err
	}
	if !hasPending {
		return shouldNotRerun, wipeDeprecatedBuckets(database)
	}

	addressesIter, sourceErr := pendingAddresses(database)
	res := migrateAddresses(ctx, database, logger, addressesIter)

	if err := sourceErr(); err != nil {
		return shouldRerun, err
	}
	if res.Err != nil || !res.IsDone {
		return shouldRerun, res.Err
	}

	return shouldNotRerun, wipeDeprecatedBuckets(database)
}

func hasPendingAddresses(r db.KeyValueReader) (bool, error) {
	prefix := db.ContractClassHash.Key()
	it, err := r.NewIterator(prefix, true)
	if err != nil {
		return false, err
	}
	defer it.Close()
	return it.First(), nil
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
			f := felt.FromBytes[felt.Felt](key[len(prefix):])
			if !yield(felt.Address(f)) {
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
