// Package statedifflength backfills BlockCommitments.StateDiffLength for blocks
// stored before that field existed.
package statedifflength

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"iter"
	"runtime"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

const (
	maxConcurrency      = 8
	batchByteSize       = 128 * db.Megabyte
	targetBatchByteSize = 96 * db.Megabyte
	timeLogRate         = 30 * time.Second
	// intermediateStateSize is the encoded resume checkpoint: one big-endian uint64.
	intermediateStateSize = 8
)

var _ migration.Migration = (*Migrator)(nil)

// Migrator reads each block's state update, computes StateDiff.Length(), and
// rewrites the block's commitments with it. Pre-existing blocks stored 0 for the
// missing field, which this replaces with the real value.
//
// Work is fanned out across reader goroutines that each fill their own write
// batch; a single committer drains them to disk. On a graceful shutdown the
// committed blocks form a contiguous range — the source emits in order and the
// pipeline drains everything it emitted — so the migration checkpoints the next
// block to resume from. After an error or crash it restarts from the oldest
// retained block, which is safe because re-deriving a length is idempotent.
type Migrator struct {
	nextBlock uint64
}

// Before restores the resume checkpoint from persisted intermediate state. A
// nil/empty state means a fresh run.
func (m *Migrator) Before(state []byte) error {
	if len(state) == 0 {
		m.nextBlock = 0
		return nil
	}
	if len(state) != intermediateStateSize {
		return fmt.Errorf("statedifflength: invalid intermediate state size: got %d, want %d",
			len(state), intermediateStateSize)
	}
	m.nextBlock = binary.BigEndian.Uint64(state)
	return nil
}

func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	_ *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	height, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil // empty database, nothing to backfill
		}
		return nil, fmt.Errorf("getting chain height: %w", err)
	}

	start, oldest, err := m.startBlock(database)
	if err != nil {
		return nil, err
	}
	if start > height {
		return nil, nil
	}

	concurrency := min(runtime.GOMAXPROCS(0), maxConcurrency)

	logger.Info("Backfilling state diff length",
		zap.Uint64("fromBlock", start),
		zap.Uint64("toBlock", height),
		zap.Int("workers", concurrency),
	)

	batchSemaphore := semaphore.New(concurrency+1, func() db.Batch {
		return database.NewBatchWithSize(batchByteSize)
	})

	progressTracker := progresslogger.NewBlockProgressTracker(
		"statedifflength", logger, height-oldest+1, start-oldest,
	)
	loggerCancel := progresslogger.CallEveryInterval(ctx, timeLogRate, progressTracker.LogProgress)
	defer loggerCancel()

	nextBlock := start
	source := pipeline.Source(blockRange(start, height, &nextBlock))
	readers := pipeline.New(
		source,
		concurrency,
		newIngestor(database, batchSemaphore, concurrency, progressTracker),
	)
	committed := pipeline.New(readers, 1, newCommitter(batchSemaphore))

	_, wait := committed.Run(ctx)
	res := wait()

	if res.Err != nil {
		return nil, res.Err
	}
	if !res.IsDone {
		logger.Info("Backfilling state diff length interrupted",
			zap.Uint64("resumeFrom", nextBlock),
		)
		return encodeResume(nextBlock), nil
	}

	logger.Info("Backfilled state diff length", zap.Uint64("throughBlock", height))
	return nil, nil
}

// startBlock returns the block to start from — the higher of the resume checkpoint
// and the oldest retained block
func (m *Migrator) startBlock(r db.KeyValueReader) (start, oldest uint64, err error) {
	oldest, err = pruner.OldestRetainedBlock(r)
	if err != nil {
		return 0, 0, fmt.Errorf("finding oldest retained block: %w", err)
	}
	return max(m.nextBlock, oldest), oldest, nil
}

func encodeResume(block uint64) []byte {
	buf := make([]byte, intermediateStateSize)
	binary.BigEndian.PutUint64(buf, block)
	return buf
}

func blockRange(start, end uint64, next *uint64) iter.Seq[uint64] {
	return func(yield func(uint64) bool) {
		for n := start; n <= end; n++ {
			if !yield(n) {
				return
			}
			*next = n + 1
		}
	}
}
