package blocktransactions

import (
	"context"
	"errors"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/prefix"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/utils"
)

const (
	// batchSize is the number of blocks to migrate in a single batch.
	// Higher batch size means less DB batch write sync overhead, but more memory usage.
	// We need to make sure some specific batches with nearly a thousand transactions per block
	// still fit [batchByteSize].
	batchSize = 10

	// batchByteSize is the initially allocated size of a batch.
	// Higher batch byte size means less DB batch write sync overhead and less reallocation when
	// batch size grows, but more memory usage when multiple batches are created.
	batchByteSize = 128 * utils.Megabyte

	// targetBatchByteSize is the threshold at which a batch is written to the database.
	// Batch is flushed to the database when it reaches this size, to avoid overflow the
	// [batchByteSize] which results in reallocation.
	targetBatchByteSize = 96 * utils.Megabyte

	// ingestorCount is the number of ingestors to run concurrently. More ingestors means more
	// spawned goroutines, which may starve the committer goroutine, while less ingestors means
	// less concurrency, which may starve the committer input channel.
	ingestorCount = 4

	// timeLogRate is the rate at which we logged the write speed.
	timeLogRate = 5 * time.Second

	// logRate is the rate at which we logged the progress in block numbers.
	logRate = 100
)

type task struct {
	batch           db.Batch
	totalTxCount    int
	totalBlockCount int
}

var (
	// We return a non-nil intermediate state if we should rerun the migration.
	shouldRerun = []byte{}

	// We return a nil intermediate state if we should not rerun the migration.
	shouldNotRerun = []byte(nil)

	// We recreated the buckets to avoid the overhead of unmarshalling the transactions and receipts.
	rawTransactions = prefix.NewPrefixedBucket(
		core.TransactionsByBlockNumberAndIndexBucket.RawValue(),
		prefix.Prefix(key.Uint64, prefix.Prefix(key.Uint64, prefix.End[[]byte]())),
	)
	rawReceipts = prefix.NewPrefixedBucket(
		core.ReceiptsByBlockNumberAndIndexBucket.RawValue(),
		prefix.Prefix(key.Uint64, prefix.Prefix(key.Uint64, prefix.End[[]byte]())),
	)
)

func migrateBlockRange(
	ctx context.Context,
	database db.KeyValueStore,
	logger utils.StructuredLogger,
	firstBlock,
	chainHeight uint64,
) pipeline.Result {
	batchSemaphore := semaphore.New(
		ingestorCount+1,
		func() db.Batch {
			return database.NewBatchWithSize(batchByteSize)
		},
	)

	blockNumberSource := pipeline.Source(func(yield func(uint64) bool) {
		for startBlock := firstBlock; startBlock <= chainHeight; startBlock += batchSize {
			if !yield(startBlock) {
				return
			}
		}
	})

	ingestorPipeline := pipeline.New(
		blockNumberSource,
		ingestorCount,
		newIngestor(logger, database, chainHeight, batchSemaphore),
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
	network *utils.Network,
	logger utils.StructuredLogger,
) ([]byte, error) {
	chainHeight, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return shouldNotRerun, nil
		}
		return shouldRerun, err
	}

	for {
		select {
		case <-ctx.Done():
			return shouldRerun, nil
		default:
		}

		firstBlock, shouldMigrate, err := getFirstBlockToMigrate(database)
		if err != nil {
			return shouldRerun, err
		}

		if !shouldMigrate {
			logger.Info("no starting block found, exiting")
			return shouldNotRerun, clearOldBuckets(database)
		}

		res := migrateBlockRange(ctx, database, logger, firstBlock, chainHeight)
		if res.Err != nil || !res.IsDone {
			return shouldRerun, res.Err
		}
	}
}
