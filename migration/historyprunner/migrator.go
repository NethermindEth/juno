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

// Migrator prunes the history of a database that ran without pruning. Instead
// of deleting the pruned entries one by one, it copies the ones to keep aside,
// wipes whole buckets and copies them back, in resumable steps:
//
//  1. Stage history: delete the block data below the cutoff and the hash
//     lookups, then copy the history entries to keep into the scratch space.
//     Once complete, wipe the history buckets.
//  2. Restore history: copy the scratch space back into the history buckets.
//  3. Restore hash lookups: rebuild them for the kept blocks, then wipe the
//     scratch space.
type Migrator struct {
	retainedBlocks uint64        // block-count floor input; only consulted on first run
	minAge         time.Duration // wallclock floor input; only consulted on first run

	floorPinned       bool // true once the fields below are set
	oldestBlockKept   uint64
	step              uint8
	resumeHistoryFrom []byte // history key the stage or restore scan resumes from
	resumeLookupsFrom uint64 // block the hash lookup restore resumes from

	layout      layout // detected on every run, see [detectLayout]
	chainHeight uint64
}

type step struct {
	name string
	run  func(
		m *Migrator,
		ctx context.Context,
		database db.KeyValueStore,
		logger log.StructuredLogger,
	) (bool, error)
}

var steps = []step{
	{name: "stage history", run: (*Migrator).runStage},
	{name: "restore history", run: (*Migrator).runRestoreHistory},
	{name: "restore hash lookups", run: (*Migrator).runRestoreLookups},
}

// stateHeaderSize is the fixed part of the intermediate state:
//
//	[0]     step to resume
//	[1:9]   oldestBlockKept (uint64 BE), pinned on the first run so a config
//	        change between runs does not retarget the cutoff mid-flight
//	[9:17]  resumeLookupsFrom (uint64 BE)
//	[17:]   resumeHistoryFrom, empty when the scan has not started
const stateHeaderSize = 17

func encodeState(
	step uint8,
	oldestBlockKept,
	resumeLookupsFrom uint64,
	resumeHistoryFrom []byte,
) []byte {
	state := []byte{step}
	state = binary.BigEndian.AppendUint64(state, oldestBlockKept)
	state = binary.BigEndian.AppendUint64(state, resumeLookupsFrom)
	return append(state, resumeHistoryFrom...)
}

// New constructs a history pruner migrator. retainedBlocks is the number
// of blocks retained below the L1-confirmed head; the head itself is
// always retained on top. retainedBlocks == 0 keeps only the L1 head plus
// the reorg-safe window above it. minAge layers a wallclock floor on top:
// blocks whose on-chain timestamp is younger than minAge are also retained,
// mirroring the running pruner's --prune-min-age. Zero disables it.
func New(retainedBlocks uint64, minAge time.Duration) *Migrator {
	return &Migrator{
		retainedBlocks: retainedBlocks,
		minAge:         minAge,
	}
}

// Before restores the step to resume, its progress and the pinned cutoff.
// A nil/empty state means a fresh run. Once pinned, retainedBlocks and minAge
// config changes between runs are ignored until completion.
func (m *Migrator) Before(state []byte) error {
	if len(state) == 0 {
		return nil
	}
	if len(state) < stateHeaderSize || int(state[0]) >= len(steps) {
		return fmt.Errorf("historyprunner: invalid intermediate state %x", state)
	}
	m.step = state[0]
	m.oldestBlockKept = binary.BigEndian.Uint64(state[1:9])
	m.resumeLookupsFrom = binary.BigEndian.Uint64(state[9:17])
	if len(state) > stateHeaderSize {
		m.resumeHistoryFrom = state[stateHeaderSize:]
	}
	m.floorPinned = true
	return nil
}

func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	chainHeight, err := core.ChainHeightBucket.Get(database, struct{}{})
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			// No chain data yet; the rolling pruner takes over once blocks arrive.
			return nil, nil
		}
		return nil, fmt.Errorf("getting chain height: %w", err)
	}
	if !m.floorPinned {
		if pruning, err := m.pinFloor(database, chainHeight); err != nil || !pruning {
			return nil, err
		}
	}
	if m.layout, err = detectLayout(database); err != nil {
		return nil, err
	}
	m.chainHeight = chainHeight

	start := time.Now()
	logger.Info("Starting history pruning migration",
		zap.Uint64("chain_height", chainHeight),
		zap.Uint64("oldest_block_kept", m.oldestBlockKept),
		zap.Stringer("layout", m.layout),
		zap.String("step", steps[m.step].name),
	)
	for ; int(m.step) < len(steps); m.step++ {
		s := steps[m.step]
		done, err := s.run(m, ctx, database, logger)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.name, err)
		}
		if !done {
			logger.Info("History pruning migration interrupted", zap.String("step", s.name))
			return encodeState(m.step, m.oldestBlockKept, m.resumeLookupsFrom, m.resumeHistoryFrom), nil
		}
	}
	logger.Info("History pruning migration completed", zap.Duration("elapsed", time.Since(start)))
	return nil, nil
}

// pinFloor computes the cutoff on the first run. It returns false when there
// is nothing below the retention window.
func (m *Migrator) pinFloor(database db.KeyValueStore, chainHeight uint64) (bool, error) {
	l1Head, err := core.L1HeightBucket.Get(database, struct{}{})
	if err != nil {
		return false, fmt.Errorf("getting L1 head: %w", err)
	}
	// pivot is the highest block eligible to be considered kept;
	// the retention floor is computed relative to it.
	pivot := min(l1Head.BlockNumber, chainHeight)
	if pivot < m.retainedBlocks {
		return false, nil
	}
	floor, err := m.retentionFloorWithMinAge(database, pivot)
	if err != nil {
		return false, fmt.Errorf("computing oldest block kept: %w", err)
	}
	if floor == 0 {
		return false, nil
	}
	m.oldestBlockKept, m.floorPinned = floor, true
	return true, nil
}

// runStage deletes the block data below the cutoff and the hash lookups, both
// idempotent and only run before the restore steps, then stages the history
// to keep. Once staged, it wipes the history buckets.
func (m *Migrator) runStage(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
) (bool, error) {
	batch := database.NewBatch()
	if err := pruner.PruneBlockDataUpto(batch, m.oldestBlockKept); err != nil {
		return false, fmt.Errorf("pruning block data upto %d: %w", m.oldestBlockKept, err)
	}
	if err := wipeReverseLookupBuckets(batch); err != nil {
		return false, fmt.Errorf("wiping reverse lookup buckets: %w", err)
	}
	if err := batch.Write(); err != nil {
		return false, fmt.Errorf("writing batch: %w", err)
	}

	next, err := stageHistory(ctx, database, m.layout, m.oldestBlockKept, m.resumeHistoryFrom, logger)
	if err != nil {
		return false, err
	}
	m.resumeHistoryFrom = next
	if next != nil {
		return false, nil
	}
	return true, m.wipeHistory(database)
}

// wipeHistory wipes the history buckets and seeds the hash lookup of the block
// below the cutoff, which StateAtBlockHash of the oldest kept block's parent
// resolves.
func (m *Migrator) wipeHistory(database db.KeyValueStore) error {
	batch := database.NewBatch()
	for _, bucket := range m.layout.buckets() {
		if err := wipeBucket(batch, byte(bucket)); err != nil {
			return err
		}
	}
	parent := m.oldestBlockKept - 1
	hash, err := core.GetBlockHeaderHashByNumber(database, parent)
	if err != nil {
		return fmt.Errorf("getting block hash at %d: %w", parent, err)
	}
	if err := core.BlockHeaderNumbersByHashBucket.Put(batch, hash, &parent); err != nil {
		return fmt.Errorf("writing block header number by hash at %d: %w", parent, err)
	}
	return batch.Write()
}

func (m *Migrator) runRestoreHistory(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
) (bool, error) {
	counter := newScanCounter(logger, "restore history", "restored")
	next, err := restoreHistory(ctx, database, m.layout, m.resumeHistoryFrom, &counter)
	if err != nil {
		return false, err
	}
	m.resumeHistoryFrom = next
	if next != nil {
		return false, nil
	}
	m.resumeLookupsFrom = m.oldestBlockKept
	return true, nil
}

// runRestoreLookups rebuilds the hash lookups of every kept block. On cancel
// the pipeline finishes the blocks it handed out, so the resume point only
// ever points past written work. Once complete, it wipes the scratch space.
func (m *Migrator) runRestoreLookups(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
) (bool, error) {
	progressTracker := progresslogger.NewBlockProgressTracker(
		"restore hash lookups",
		logger,
		m.chainHeight-m.oldestBlockKept+1,
		m.resumeLookupsFrom-m.oldestBlockKept,
	)
	loggerCancel := progresslogger.CallEveryInterval(ctx, timeLogRate, progressTracker.LogProgress)
	defer loggerCancel()

	maxWorkers := runtime.GOMAXPROCS(0)
	batchSemaphore := semaphore.New(
		maxWorkers+1,
		func() db.Batch {
			return database.NewBatchWithSize(int(batchByteSize))
		},
	)
	next, err := migrateRange(
		ctx,
		logger,
		batchSemaphore,
		m.resumeLookupsFrom,
		m.chainHeight,
		maxWorkers,
		newRestorer(database, batchSemaphore, maxWorkers, progressTracker),
	)
	if err != nil {
		return false, fmt.Errorf("restoring range [%d, %d]: %w", m.resumeLookupsFrom, m.chainHeight, err)
	}
	m.resumeLookupsFrom = next
	if next <= m.chainHeight {
		return false, nil
	}

	batch := database.NewBatch()
	if err := wipeScratchSpace(batch); err != nil {
		return false, fmt.Errorf("wiping scratch space: %w", err)
	}
	return true, batch.Write()
}

// newScanCounter logs the progress of a history scan, which has no total.
func newScanCounter(logger log.StructuredLogger, name, unit string) progresslogger.Counter {
	counter := progresslogger.NewCounter(logger, timeLogRate, name)
	counter.SetProgress(unit, 0, 0)
	return counter
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

// retentionFloorWithMinAge returns min(standardFloor, minAgeFloor).
// Precondition: pivot >= retainedBlocks.
func (m *Migrator) retentionFloorWithMinAge(
	database db.KeyValueStore,
	pivot uint64,
) (uint64, error) {
	standardFloor := pivot - m.retainedBlocks
	if m.minAge == 0 {
		return standardFloor, nil
	}
	minAgeFloor, err := pruner.FindOldestBlockAtOrAfter(database, 0, pivot, time.Now().Add(-m.minAge))
	if errors.Is(err, pruner.ErrNoBlockInWindow) {
		// Deep catch-up: no block young enough; wallclock layer disabled.
		return standardFloor, nil
	}
	if err != nil {
		return 0, err
	}
	return min(standardFloor, minAgeFloor), nil
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

func wipeBucket(batch db.Batch, bucket byte) error {
	prefix := []byte{bucket}
	end := dbutils.UpperBound(prefix)
	if err := batch.DeleteRange(prefix, end); err != nil {
		return fmt.Errorf("deleting range bucket=%d: %w", bucket, err)
	}
	return nil
}
