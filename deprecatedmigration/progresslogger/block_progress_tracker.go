package progresslogger

import (
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/utils"
)

// BlockProgressTracker tracks the progress of a migration by block number.
type BlockProgressTracker struct {
	logger          utils.StructuredLogger
	totalBlocks     uint64
	completedBlocks atomic.Uint64
	startTimestamp  time.Time
}

// NewBlockProgressTracker creates a new progress tracker for block-based migrations.
// initialCompletedBlocks allows resuming from a previous migration state.
func NewBlockProgressTracker(
	logger utils.StructuredLogger,
	totalBlocks uint64,
	initialCompletedBlocks uint64,
) *BlockProgressTracker {
	tracker := &BlockProgressTracker{
		logger:         logger,
		totalBlocks:    totalBlocks,
		startTimestamp: time.Now(),
	}
	if initialCompletedBlocks > 0 {
		tracker.completedBlocks.Store(initialCompletedBlocks)
	}
	return tracker
}

// IncrementCompletedBlocks increments the completed blocks counter by the specified amount.
// Safe to call from multiple goroutines concurrently.
func (t *BlockProgressTracker) IncrementCompletedBlocks(amount uint64) {
	t.completedBlocks.Add(amount)
}

// LogProgress logs the progress of the migration.
func (t *BlockProgressTracker) LogProgress() {
	percentage := float64(t.completedBlocks.Load()) / float64(t.totalBlocks) * 100
	t.logger.Info(
		"Migration progress",
		utils.SugaredFields(
			"percentage", percentage,
			"elapsed", t.Elapsed(),
		)...,
	)
}

func (t *BlockProgressTracker) Elapsed() time.Duration {
	return time.Since(t.startTimestamp)
}
