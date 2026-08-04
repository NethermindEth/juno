package statedifflength

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/pruner"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

const (
	defaultBatchByteSize = 16 * db.Megabyte
	timeLogRate          = 5 * time.Second
)

var _ migration.Migration = (*Migrator)(nil)

// Migrator walks every stored block and, where the commitments record predates
// StateDiffLength, reads the state update once to compute the length and rewrites
// the commitments with it
//
// Resumable: the next block to visit is carried in the intermediate state, so an
// interrupted run continues where it stopped. Re-run safe: blocks that already
// have a length are skipped.
type Migrator struct {
	// BatchByteSize overrides the write batch size. Zero uses the default.
	BatchByteSize int

	nextBlock uint64
}

func (m *Migrator) Before(state []byte) error {
	if len(state) == 0 {
		m.nextBlock = 0
		return nil
	}
	if len(state) != 8 {
		return fmt.Errorf("malformed intermediate state: len %d, want 8", len(state))
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

	start, err := m.startBlock(database)
	if err != nil {
		return nil, err
	}
	if start > height {
		return nil, nil
	}

	logger.Info("Backfilling state diff length",
		zap.Uint64("fromBlock", start),
		zap.Uint64("toBlock", height),
	)

	batchByteSize := m.BatchByteSize
	if batchByteSize == 0 {
		batchByteSize = defaultBatchByteSize
	}

	batch := database.NewBatchWithSize(batchByteSize)
	lastLog := time.Now()
	var updated uint64

	for blockNumber := start; blockNumber <= height; blockNumber++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			if err := m.commit(batch, blockNumber); err != nil {
				return nil, err
			}
			return m.state(), ctxErr
		}

		done, err := backfillBlock(database, batch, blockNumber)
		if err != nil {
			return nil, fmt.Errorf("backfilling block %d: %w", blockNumber, err)
		}
		if done {
			updated++
		}

		if batch.Size() >= batchByteSize {
			if err := m.commit(batch, blockNumber+1); err != nil {
				return nil, err
			}
			batch = database.NewBatchWithSize(batchByteSize)
		}

		if time.Since(lastLog) >= timeLogRate {
			logger.Info("Backfilling state diff length",
				zap.Uint64("block", blockNumber),
				zap.Uint64("toBlock", height),
				zap.Uint64("updated", updated),
			)
			lastLog = time.Now()
		}
	}

	if err := batch.Write(); err != nil {
		return nil, fmt.Errorf("writing final batch: %w", err)
	}

	logger.Info("Backfilled state diff length", zap.Uint64("updated", updated))
	return nil, nil
}

// startBlock skips the pruned prefix, where every lookup would miss, without
// losing the resume point.
func (m *Migrator) startBlock(r db.KeyValueReader) (uint64, error) {
	oldest, err := pruner.OldestRetainedBlock(r)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return m.nextBlock, nil
		}
		return 0, fmt.Errorf("finding oldest retained block: %w", err)
	}
	return max(m.nextBlock, oldest), nil
}

// backfillBlock reports whether the block's commitments were rewritten.
func backfillBlock(r db.KeyValueReader, w db.KeyValueWriter, blockNumber uint64) (bool, error) {
	commitments, err := core.GetBlockCommitmentByBlockNum(r, blockNumber)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	if commitments.StateDiffLength != nil {
		return false, nil
	}

	stateUpdate, err := core.GetStateUpdateByBlockNum(r, blockNumber)
	if err != nil {
		return false, err
	}

	length := stateUpdate.StateDiff.Length()
	commitments.StateDiffLength = &length
	if err := core.WriteBlockCommitment(w, blockNumber, commitments); err != nil {
		return false, err
	}
	return true, nil
}

func (m *Migrator) commit(batch db.Batch, nextBlock uint64) error {
	if err := batch.Write(); err != nil {
		return fmt.Errorf("writing batch: %w", err)
	}
	m.nextBlock = nextBlock
	return nil
}

func (m *Migrator) state() []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], m.nextBlock)
	return buf[:]
}
