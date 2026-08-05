// Package statedifflength backfills BlockCommitments.StateDiffLength for blocks
// stored before that field existed.
package statedifflength

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"runtime"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

const (
	maxConcurrency       = 8
	defaultBatchByteSize = 24 * db.Megabyte
	targetBatchByteSize  = 16 * db.Megabyte
)

var (
	shouldRerun    = []byte{}
	shouldNotRerun []byte
)

var _ migration.Migration = (*Migrator)(nil)

// Migrator reads each block's state update, computes StateDiff.Length(), and
// rewrites the block's commitments with it. Pre-existing blocks stored 0 for the
// missing field, which this replaces with the real value.
//
// Work is fanned out across Concurrency reader goroutines that each fill their own
// write batch; a single committer drains the batches to disk. There is no resume
// cursor: an interrupted run restarts from the oldest retained block on the next
// boot, which is safe because re-deriving a length yields the same value.
type Migrator struct {
	Concurrency   int
	BatchByteSize int
}

func (m *Migrator) Before([]byte) error { return nil }

func (m *Migrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	_ *networks.Network,
	logger log.StructuredLogger,
) ([]byte, error) {
	height, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return shouldNotRerun, nil // empty database, nothing to backfill
		}
		return shouldNotRerun, fmt.Errorf("getting chain height: %w", err)
	}

	start, err := m.startBlock(database)
	if err != nil {
		return shouldNotRerun, err
	}
	if start > height {
		return shouldNotRerun, nil
	}

	concurrency := m.Concurrency
	if concurrency <= 0 {
		concurrency = min(runtime.GOMAXPROCS(0), maxConcurrency)
	}
	batchByteSize := m.BatchByteSize
	if batchByteSize <= 0 {
		batchByteSize = defaultBatchByteSize
	}

	logger.Info("Backfilling state diff length",
		zap.Uint64("fromBlock", start),
		zap.Uint64("toBlock", height),
		zap.Int("workers", concurrency),
	)

	batchSemaphore := semaphore.New(concurrency+1, func() db.Batch {
		return database.NewBatchWithSize(batchByteSize)
	})

	source := pipeline.Source(blockRange(start, height))
	readers := pipeline.New(
		source, concurrency, newIngestor(database, batchSemaphore,
			concurrency, min(batchByteSize, targetBatchByteSize)),
	)
	committed := pipeline.New(
		readers, 1, newCommitter(logger, batchSemaphore, height),
	)

	_, wait := committed.Run(ctx)
	res := wait()

	if res.Err != nil {
		return shouldRerun, res.Err
	}
	if !res.IsDone {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return shouldRerun, ctxErr
		}
		return shouldRerun, errors.New("state diff length pipeline did not complete")
	}
	return shouldNotRerun, nil
}

// startBlock skips the pruned prefix, where every lookup would miss.
func (m *Migrator) startBlock(r db.KeyValueReader) (uint64, error) {
	oldest, err := pruner.OldestRetainedBlock(r)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("finding oldest retained block: %w", err)
	}
	return oldest, nil
}

func blockRange(start, end uint64) iter.Seq[uint64] {
	return func(yield func(uint64) bool) {
		for n := start; n <= end; n++ {
			if !yield(n) {
				return
			}
		}
	}
}

// backfillBlock derives the state diff length from the block's state update and
// writes the block's commitments with it into batch. A missing commitments or
// state update record above the pruned prefix means the two buckets disagree, i.e.
// the database is corrupt, so the read error is returned rather than skipped.
func backfillBlock(r db.KeyValueReader, batch db.KeyValueWriter, blockNumber uint64) error {
	commitments, err := core.GetBlockCommitmentByBlockNum(r, blockNumber)
	if err != nil {
		return fmt.Errorf("getting commitments for block %d: %w", blockNumber, err)
	}

	stateUpdate, err := core.GetStateUpdateByBlockNum(r, blockNumber)
	if err != nil {
		return fmt.Errorf("getting state update for block %d: %w", blockNumber, err)
	}

	commitments.StateDiffLength = stateUpdate.StateDiff.Length()
	return core.WriteBlockCommitment(batch, blockNumber, commitments)
}
