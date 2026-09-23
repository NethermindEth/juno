package blockeventsbloom

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	_ "github.com/NethermindEth/juno/encoder/registry"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/progresslogger"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

const (
	// batchByteSize is the initially allocated size of the write batch.
	batchByteSize = 128 * db.Megabyte

	// targetBatchByteSize is the threshold at which the write batch is flushed to disk.
	targetBatchByteSize = 96 * db.Megabyte

	// commitQueueSize is how many full batches may wait for the committer before the walk blocks.
	commitQueueSize = 4

	// progressLogInterval is how often migration progress (percentage) is logged.
	progressLogInterval = 30 * time.Second

	// migrationName labels this migration's progress log lines.
	migrationName = "block events bloom"
)

var _ migration.Migration = (*Migrator)(nil)

// Migrator rewrites every block header without its embedded event bloom filter: event
// lookups go through the aggregated bloom filter, and a per-block bloom is rebuilt from
// receipts on demand.
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
		return nil, fmt.Errorf("getting chain height: %w", err)
	}

	if m.startFrom > 0 {
		logger.Info("Resuming block events bloom migration", zap.Uint64("fromBlock", m.startFrom))
	}

	nextBlock, err := stripHeaders(ctx, database, logger, m.startFrom, chainHeight)
	if err != nil {
		return nil, err
	}
	// Not all blocks reached: the run was interrupted. Persist the resume
	// point so the next run continues instead of rescanning from the start.
	if nextBlock <= chainHeight {
		return encodeIntermediateState(nextBlock), nil
	}
	return nil, nil
}

// stripHeaders walks the header bucket from startFrom to chainHeight in one sequential
// pass, rewriting each header without its bloom, and returns the next block still to
// migrate: chainHeight+1 when the walk completes, or the block it stopped at when the
// context was cancelled. Every block before the returned value is committed. On a
// pruning node the walk starts at the first retained header, whatever startFrom is;
// from there the headers must be contiguous, and a gap is an error.
func stripHeaders(
	ctx context.Context,
	database db.KeyValueStore,
	logger log.StructuredLogger,
	startFrom,
	chainHeight uint64,
) (nextBlock uint64, err error) {
	it, err := database.NewIterator(db.BlockHeadersByNumber.Key(), true)
	if err != nil {
		return 0, fmt.Errorf("opening block header iterator: %w", err)
	}
	defer it.Close()

	if !it.Seek(db.BlockHeaderByNumberKey(startFrom)) {
		return chainHeight + 1, nil
	}
	firstBlock := blockNumberFromKey(it.Key())

	tracker := progresslogger.NewBlockProgressTracker(migrationName, logger, chainHeight-firstBlock+1, 0)
	stopLog := progresslogger.CallEveryInterval(ctx, progressLogInterval, tracker.LogProgress)
	defer stopLog()
	defer tracker.LogProgress()

	// A committer failure cancels walkCtx so the walk stops on its next header. Closing
	// the queue stops the committer; every exit path then waits for it and joins its
	// error with the walk's.
	commitQueue := make(chan db.Batch, commitQueueSize)
	committer, walkCtx := errgroup.WithContext(ctx)
	committer.Go(func() error { return commitBatches(logger, commitQueue) })
	defer func() {
		close(commitQueue)
		err = errors.Join(err, committer.Wait())
	}()

	batch := database.NewBatchWithSize(batchByteSize)
	nextBlock = firstBlock
	for valid := true; valid && walkCtx.Err() == nil; valid = it.Next() {
		blockNumber := blockNumberFromKey(it.Key())
		if blockNumber != nextBlock {
			return 0, fmt.Errorf("missing block header %d: next stored header is %d", nextBlock, blockNumber)
		}

		value, err := it.UncopiedValue()
		if err != nil {
			return 0, fmt.Errorf("reading block header %d: %w", blockNumber, err)
		}
		// Decoding into the bloom-less core.Header drops the legacy bloom field.
		var header core.Header
		if err := encoder.Unmarshal(value, &header); err != nil {
			return 0, fmt.Errorf("decoding block header %d: %w", blockNumber, err)
		}
		if err := core.WriteBlockHeaderByNumber(batch, &header); err != nil {
			return 0, fmt.Errorf("writing block header %d: %w", blockNumber, err)
		}
		nextBlock = blockNumber + 1
		tracker.IncrementCompletedBlocks(1)

		if batch.Size() >= targetBatchByteSize {
			commitQueue <- batch // blocks once the queue is full
			batch = database.NewBatchWithSize(batchByteSize)
		}
	}

	commitQueue <- batch
	if walkCtx.Err() == nil && nextBlock <= chainHeight {
		return 0, fmt.Errorf("missing block header %d: header bucket ends before chain height %d", nextBlock, chainHeight)
	}
	return nextBlock, nil
}

// commitBatches writes each queued batch in order until the queue closes. After a failure
// it keeps draining so the walk never blocks, and returns the first error.
func commitBatches(logger log.StructuredLogger, commitQueue <-chan db.Batch) error {
	var firstErr error
	for batch := range commitQueue {
		if firstErr != nil {
			continue
		}
		logger.Debug("Writing batch", zap.Int("batchSize", batch.Size()))
		if err := batch.Write(); err != nil {
			firstErr = fmt.Errorf("writing block header batch: %w", err)
		}
	}
	return firstErr
}

// blockNumberFromKey extracts the block number from a BlockHeadersByNumber key, which
// the prefix-bounded iterator guarantees is the bucket byte followed by 8 big-endian bytes.
func blockNumberFromKey(key []byte) uint64 {
	return binary.BigEndian.Uint64(key[1:])
}

func encodeIntermediateState(nextBlock uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, nextBlock)
	return buf
}
