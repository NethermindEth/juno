package historyprunner

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

const (
	// batchByteSize is the initially allocated size of a batch.
	batchByteSize = 128 * db.Megabyte

	// targetBatchByteSize is the threshold at which a batch is written to the database.
	targetBatchByteSize = 96 * db.Megabyte

	// timeLogRate is the rate at which we log the progress.
	timeLogRate = 30 * time.Second
)

var _ migration.Migration = (*Migrator)(nil)

// intermediateStateSize is the on-disk encoding length:
//
//	[0:8]   stagerProgress    (uint64 BE)
//	[8:16]  restorerProgress  (uint64 BE)
//	[16:24] numRetainedBlocks (uint64 BE) — pinned from the first run so that
//	                                       a config change between runs does
//	                                       not retarget the cutoff mid-flight.
const intermediateStateSize = 24

type Migrator struct {
	numRetainedBlocks uint64 // active value; overridden by Before from persisted state on resume
	configured        uint64 // value passed to New (what the running process is configured for)
	stagerProgress    uint64
	restorerProgress  uint64
}

// New constructs a history pruner migrator. retainedBlocks is the number
// of blocks retained below the L1-confirmed head; the head itself is
// always retained on top. retainedBlocks == 0 keeps only the L1 head plus
// the reorg-safe window above it.
func New(retainedBlocks uint64) *Migrator {
	return &Migrator{
		numRetainedBlocks: retainedBlocks,
		configured:        retainedBlocks,
	}
}

// Before restores stager/restorer progress and the originally-configured retention
// window from the persisted intermediate state. A nil/empty state means a
// fresh run — the constructor defaults stand. If retainedBlocks was changed
// between runs, the persisted (original) value wins; Migrate logs the
// divergence so the operator can see why the new value is being ignored.
func (m *Migrator) Before(state []byte) error {
	if len(state) == 0 {
		return nil
	}
	if len(state) != intermediateStateSize {
		return fmt.Errorf("historyprunner: invalid intermediate state size: got %d, want %d",
			len(state),
			intermediateStateSize,
		)
	}
	m.stagerProgress = binary.BigEndian.Uint64(state[0:8])
	m.restorerProgress = binary.BigEndian.Uint64(state[8:16])
	m.numRetainedBlocks = binary.BigEndian.Uint64(state[16:24])
	return nil
}

func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	if m.configured != m.numRetainedBlocks {
		logger.Info("Resuming pruning migration; new retention ignored until completion",
			zap.Uint64("active_retained_blocks", m.numRetainedBlocks),
			zap.Uint64("requested_retained_blocks", m.configured),
		)
	}

	chainHeight, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// No chain data yet; the rolling pruner takes over once blocks arrive.
			return nil, nil
		}
		return nil, fmt.Errorf("getting chain height: %w", err)
	}
	l1Head, err := core.GetL1Head(database)
	if err != nil {
		return nil, fmt.Errorf("getting L1 head: %w", err)
	}
	minHead := min(l1Head.BlockNumber, chainHeight)
	if minHead < m.numRetainedBlocks {
		// Chain shorter than the retention window — nothing to prune yet.
		return nil, nil
	}
	oldestBlockKept := minHead - m.numRetainedBlocks

	start := time.Now()
	logger.Info("Starting history pruning migration",
		zap.Uint64("chain_height", chainHeight),
		zap.Uint64("oldest_block_kept", oldestBlockKept),
		zap.Uint64("l1_head", l1Head.BlockNumber),
	)

	if err := m.setupBeforeStager(database, oldestBlockKept); err != nil {
		return nil, fmt.Errorf("setting up before stager: %w", err)
	}

	maxWorkers := runtime.GOMAXPROCS(0)
	batchSemaphore := semaphore.New(
		maxWorkers+1,
		func() db.Batch {
			return database.NewBatchWithSize(int(batchByteSize))
		},
	)
	state, done, err := m.runStager(
		ctx,
		database,
		batchSemaphore,
		logger,
		oldestBlockKept,
		chainHeight,
		maxWorkers,
	)
	if err != nil {
		return nil, fmt.Errorf("running stager: %w", err)
	}
	if !done {
		return state, nil
	}

	if err := m.setupBeforeRestorer(database); err != nil {
		return nil, fmt.Errorf("setting up before restorer: %w", err)
	}

	state, done, err = m.runRestorer(
		ctx,
		database,
		batchSemaphore,
		logger,
		oldestBlockKept,
		chainHeight,
		maxWorkers,
	)
	if err != nil {
		return nil, fmt.Errorf("running restorer: %w", err)
	}
	if !done {
		return state, nil
	}

	logger.Info("History pruning migration completed", zap.Duration("elapsed", time.Since(start)))
	return nil, nil
}

// setupBeforeStager range-deletes the cold number-keyed buckets and wipes the
// reverse-lookup buckets. Gated by restorerProgress because the restorer rebuilds
// the reverse-lookup buckets — re-running this after the restorer has started would
// delete restored data and corrupt the db.
func (m *Migrator) setupBeforeStager(
	database db.KeyValueStore,
	oldestBlockKept uint64,
) error {
	if m.restorerProgress != 0 {
		return nil
	}
	batch := database.NewBatch()
	if err := pruner.PruneBlockDataUpto(batch, oldestBlockKept); err != nil {
		return fmt.Errorf("pruning block data upto %d: %w", oldestBlockKept, err)
	}
	if err := wipeReverseLookupBuckets(batch); err != nil {
		return fmt.Errorf("wiping reverse lookup buckets: %w", err)
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("writing batch: %w", err)
	}
	return nil
}

// setupBeforeRestorer wipes the live history buckets so the per-block restore
// can copy staged keepers back. Gated by restorerProgress so a resumed
// restorer doesn't re-wipe partial work.
func (m *Migrator) setupBeforeRestorer(database db.KeyValueStore) error {
	if m.restorerProgress != 0 {
		return nil
	}
	batch := database.NewBatch()
	if err := wipeStorageHistoryBuckets(batch); err != nil {
		return fmt.Errorf("wiping storage history buckets: %w", err)
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("writing batch: %w", err)
	}
	return nil
}

// runStager stages the keeper-window history into the scratch namespace.
// The first invocation also range-deletes the cold (pre-cutoff) number-keyed
// buckets and wipes the reverse-lookup buckets; they remain empty until
// runRestorer rebuilds them.
//
// done=false signals the stager was interrupted (state is the resume blob);
// done=true signals completion.
func (m *Migrator) runStager(
	ctx context.Context,
	database db.KeyValueStore,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	logger log.StructuredLogger,
	oldestBlockKept, chainHeight uint64,
	maxWorkers int,
) ([]byte, bool, error) {
	if m.stagerProgress > chainHeight {
		logger.Info("Stager already completed in a previous run, skipping")
		return nil, true, nil
	}
	m.stagerProgress = max(oldestBlockKept, m.stagerProgress)

	// Progress is reported relative to the keeper window [oldestBlockKept,
	// chainHeight] — passing absolute block numbers would start the readout
	// near 100% and never visibly advance.
	keeperWindow := chainHeight - oldestBlockKept + 1
	progressTracker := progresslogger.NewBlockProgressTracker(
		"stager", logger, keeperWindow, m.stagerProgress-oldestBlockKept,
	)
	loggerCancel := progresslogger.CallEveryInterval(ctx, timeLogRate, progressTracker.LogProgress)
	defer loggerCancel()

	resumeFrom, err := migrateRange(
		ctx,
		logger,
		batchSemaphore,
		m.stagerProgress,
		chainHeight,
		maxWorkers,
		newStager(database, batchSemaphore, maxWorkers, progressTracker),
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"staging range [%d, %d]: %w",
			m.stagerProgress, chainHeight, err,
		)
	}

	if resumeFrom <= chainHeight {
		logger.Info("Stager interrupted",
			zap.Uint64("resume_from", resumeFrom),
		)
		return encodeIntermediateState(resumeFrom, 0, m.numRetainedBlocks), false, nil
	}
	return nil, true, nil
}

// runRestorer wipes the live history buckets, then for each keeper block
// copies its state history back from scratch and rebuilds the
// (hash → block-number) reverse-lookup indices. On successful completion
// the scratch namespace is wiped before returning.
func (m *Migrator) runRestorer(
	ctx context.Context,
	database db.KeyValueStore,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	logger log.StructuredLogger,
	oldestBlockKept,
	chainHeight uint64,
	maxWorkers int,
) ([]byte, bool, error) {
	m.restorerProgress = max(oldestBlockKept, m.restorerProgress)

	keeperWindow := chainHeight - oldestBlockKept + 1
	progressTracker := progresslogger.NewBlockProgressTracker(
		"restorer", logger, keeperWindow, m.restorerProgress-oldestBlockKept,
	)
	loggerCancel := progresslogger.CallEveryInterval(ctx, timeLogRate, progressTracker.LogProgress)
	defer loggerCancel()

	header, err := core.GetBlockHeaderByNumber(database, oldestBlockKept-uint64(1))
	if err != nil {
		return nil, false, fmt.Errorf("getting block header at %d: %w", oldestBlockKept-1, err)
	}

	batch := database.NewBatch()
	err = core.WriteBlockHeaderNumberByHash(batch, header.Hash, oldestBlockKept-uint64(1))
	if err != nil {
		return nil, false, fmt.Errorf(
			"writing block header number by hash at %d: %w",
			oldestBlockKept-1, err,
		)
	}
	if err := batch.Write(); err != nil {
		return nil, false, fmt.Errorf("writing reverse-lookup seed batch: %w", err)
	}

	resumeFrom, err := migrateRange(
		ctx,
		logger,
		batchSemaphore,
		m.restorerProgress,
		chainHeight,
		maxWorkers,
		newRestorer(database, batchSemaphore, maxWorkers, progressTracker),
	)
	if err != nil {
		return nil, false, fmt.Errorf(
			"restoring range [%d, %d]: %w",
			m.restorerProgress, chainHeight, err,
		)
	}

	if resumeFrom <= chainHeight {
		logger.Info("Restorer interrupted",
			zap.Uint64("resume_from", resumeFrom),
		)
		return encodeIntermediateState(chainHeight+1, resumeFrom, m.numRetainedBlocks), false, nil
	}

	// Restorer completed: wipe the scratch namespace.
	batch = database.NewBatch()
	if err := wipeScratchSpace(batch); err != nil {
		return nil, false, fmt.Errorf("wiping scratch space: %w", err)
	}
	if err := batch.Write(); err != nil {
		return nil, false, fmt.Errorf("writing scratch-wipe batch: %w", err)
	}
	return nil, true, nil
}

func migrateRange(
	ctx context.Context,
	logger log.StructuredLogger,
	batchSemaphore semaphore.ResourceSemaphore[db.Batch],
	fromBlock,
	toBlock uint64,
	maxWorkers int,
	processor pipeline.State[uint64, db.Batch],
) (uint64, error) {
	nextBlockNumber := fromBlock
	blockNumberSource := pipeline.Source(func(yield func(uint64) bool) {
		for ; nextBlockNumber <= toBlock; nextBlockNumber++ {
			if !yield(nextBlockNumber) {
				return
			}
		}
	})

	processorPipeline := pipeline.New(blockNumberSource, maxWorkers, processor)
	committerPipeline := pipeline.New(
		processorPipeline,
		maxWorkers,
		newCommitter(logger, batchSemaphore),
	)

	_, wait := committerPipeline.Run(ctx)
	if res := wait(); res.Err != nil {
		return 0, fmt.Errorf("running pipeline: %w", res.Err)
	}

	return nextBlockNumber, nil
}

func encodeIntermediateState(stagerCompletion, restorerCompletion, retainedBlocks uint64) []byte {
	buf := make([]byte, intermediateStateSize)
	binary.BigEndian.PutUint64(buf[0:8], stagerCompletion)
	binary.BigEndian.PutUint64(buf[8:16], restorerCompletion)
	binary.BigEndian.PutUint64(buf[16:24], retainedBlocks)
	return buf
}

func wipeScratchSpace(batch db.Batch) error {
	return wipeBucket(batch, migrationScratchTag)
}

func wipeReverseLookupBuckets(batch db.Batch) error {
	if err := wipeBucket(batch, byte(db.TransactionBlockNumbersAndIndicesByHash)); err != nil {
		return fmt.Errorf("wiping TransactionBlockNumbersAndIndicesByHash: %w", err)
	}

	if err := wipeBucket(batch, byte(db.L1HandlerTxnHashByMsgHash)); err != nil {
		return fmt.Errorf("wiping L1HandlerTxnHashByMsgHash: %w", err)
	}

	if err := wipeBucket(batch, byte(db.BlockHeaderNumbersByHash)); err != nil {
		return fmt.Errorf("wiping BlockHeaderNumbersByHash: %w", err)
	}
	return nil
}

func wipeStorageHistoryBuckets(batch db.Batch) error {
	if err := wipeBucket(batch, byte(db.ContractStorageHistory)); err != nil {
		return fmt.Errorf("wiping ContractStorageHistory: %w", err)
	}

	if err := wipeBucket(batch, byte(db.ContractClassHashHistory)); err != nil {
		return fmt.Errorf("wiping ContractClassHashHistory: %w", err)
	}

	if err := wipeBucket(batch, byte(db.ContractNonceHistory)); err != nil {
		return fmt.Errorf("wiping ContractNonceHistory: %w", err)
	}
	return nil
}

func wipeBucket(batch db.Batch, bucket byte) error {
	prefix := []byte{bucket}
	end := dbutils.UpperBound(prefix)
	if err := batch.DeleteRange(prefix, end); err != nil {
		return fmt.Errorf("deleting range bucket=%d: %w", bucket, err)
	}
	return nil
}
