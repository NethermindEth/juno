package blockeventsbloom

import (
	"context"
	"encoding/binary"
	"errors"
	"runtime"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

const (
	// batchSize is the number of blocks migrated per source chunk.
	batchSize = 100

	// batchByteSize is the initially allocated size of a batch.
	batchByteSize = 128 * db.Megabyte

	// targetBatchByteSize is the threshold at which a batch is flushed to disk.
	targetBatchByteSize = 96 * db.Megabyte

	// progressLogInterval is how often migration progress (percentage) is logged.
	progressLogInterval = 30 * time.Second

	// migrationName labels this migration's progress log lines.
	migrationName = "block events bloom"
)

// migrateBlockRange migrates [firstBlock, chainHeight] and returns the next
// block that still needs migrating. When the range completes this is
// chainHeight+1; when the run is interrupted (context cancelled) it is the
// block the source stopped at. Because the pipeline uses unbuffered channels
// and flushes pending batches on Done, every block before the returned value
// is committed before this returns, so it is a safe resume point.
func migrateBlockRange(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
	tracker *progresslogger.BlockProgressTracker,
	firstBlock,
	chainHeight uint64,
) (uint64, error) {
	loggerCancel := progresslogger.CallEveryInterval(ctx, progressLogInterval, tracker.LogProgress)
	defer loggerCancel()

	ingestorCount := runtime.GOMAXPROCS(0)
	batchSemaphore := semaphore.New(
		ingestorCount+1,
		func() db.Batch {
			return database.NewBatchWithSize(batchByteSize)
		},
	)

	nextStartBlock := firstBlock
	blockNumberSource := pipeline.Source(func(yield func(uint64) bool) {
		for ; nextStartBlock <= chainHeight; nextStartBlock += batchSize {
			if !yield(nextStartBlock) {
				return
			}
		}
	})

	ingestorPipeline := pipeline.New(
		blockNumberSource,
		ingestorCount,
		newIngestor(database, chainHeight, batchSemaphore, tracker, ingestorCount),
	)

	committerPipeline := pipeline.New(
		ingestorPipeline,
		1,
		newCommitter(logger, batchSemaphore),
	)

	_, wait := committerPipeline.Run(ctx)
	return nextStartBlock, wait().Err
}

var _ migration.Migration = (*Migrator)(nil)

// Migrator strips each block's event bloom filter out of the block header,
// rewriting the header without the bloom. The bloom is discarded — it is
// reconstructable on demand from the block's receipts.
type Migrator struct {
	// startFrom is the block to resume migrating from, restored from the
	// intermediate state of a previous interrupted run.
	startFrom uint64
}

// Before restores the resume point saved by a previous interrupted run.
func (m *Migrator) Before(intermediateState []byte) error {
	if len(intermediateState) >= 8 {
		m.startFrom = binary.BigEndian.Uint64(intermediateState[:8])
	}
	return nil
}

func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	_ *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	chainHeight, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}

	// Start at the retention floor so pruning nodes don't scan block numbers
	// whose data (and headers) have been pruned away. On non-pruning nodes this
	// is genesis (0). A resume point from a previous run takes precedence when
	// it is ahead of the floor.
	firstBlock, err := pruner.OldestRetainedBlock(database)
	if err != nil {
		return nil, err
	}
	startFrom := max(firstBlock, m.startFrom)

	if startFrom > firstBlock {
		logger.Info("Resuming block events bloom migration", zap.Uint64("fromBlock", startFrom))
	}

	// totalBlocks spans [0, chainHeight]; startFrom seeds the completed count so
	// the percentage stays meaningful across resumes and on pruning nodes.
	tracker := progresslogger.NewBlockProgressTracker(migrationName, logger, chainHeight, startFrom)

	resumeFrom, err := migrateBlockRange(ctx, database, logger, tracker, startFrom, chainHeight)
	if err != nil {
		return nil, err
	}
	tracker.LogProgress()
	// Not all blocks reached: the run was interrupted. Persist the resume
	// point so the next run continues instead of rescanning from the floor.
	if resumeFrom <= chainHeight {
		return encodeIntermediateState(resumeFrom), nil
	}
	return nil, nil
}

func encodeIntermediateState(nextBlock uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, nextBlock)
	return buf
}
