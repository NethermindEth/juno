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
	logger utils.SimpleLogger,
	firstBlock,
	chainHeight uint64,
) error {
	batchProvider := newBatchProvider(database, ingestorCount+1)
	defer batchProvider.close()

	startBlockCh := make(chan uint64)

	ingestorPipeline := newPipeline(
		startBlockCh,
		ingestorCount,
		newIngestor(logger, database, chainHeight, batchProvider),
	)

	committerPipeline := newPipeline(
		ingestorPipeline.outputs,
		1,
		newCommitter(logger, batchProvider),
	)

outerLoop:
	for startBlock := firstBlock; startBlock <= chainHeight; startBlock += batchSize {
		select {
		case <-ctx.Done():
			break outerLoop
		case startBlockCh <- startBlock:
		}
	}
	close(startBlockCh)

	// Wait for all ingestors to finish, because we closed their inputs [startBlockCh].
	// Then their output channel is closed
	if err := ingestorPipeline.wait(); err != nil {
		return err
	}

	// Wait for the committer to finish, because we closed its input [ingestorPipeline.outputs].
	// Then its output channel is closed
	return committerPipeline.wait()
}

type BlockTransactionsMigration struct{}

func (BlockTransactionsMigration) Before([]byte) error {
	return nil
}

func (BlockTransactionsMigration) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *utils.Network,
	logger utils.SimpleLogger,
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
			logger.Infow("no starting block found, exiting")
			return shouldNotRerun, clearOldBuckets(database)
		}

		if err := migrateBlockRange(ctx, database, logger, firstBlock, chainHeight); err != nil {
			return shouldRerun, err
		}
	}
}
