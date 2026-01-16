package progresslogger

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/NethermindEth/juno/utils"
)

// BlockNumberProgressTracker tracks the progress of a migration by block number.
type BlockNumberProgressTracker struct {
	logger          utils.SimpleLogger //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	totalBlocks     uint64
	completedBlocks atomic.Uint64
	startTimestamp  time.Time
}

// NewBlockNumberProgressTracker creates a new progress tracker for block-based migrations.
// initialCompletedBlocks allows resuming from a previous migration state.
func NewBlockNumberProgressTracker(
	logger utils.SimpleLogger, //nolint:staticcheck,nolintlint,lll // ignore staticcheck we are complying with the Migration interface, nolintlint because main config does not checks
	totalBlocks uint64,
	initialCompletedBlocks uint64,
) *BlockNumberProgressTracker {
	tracker := &BlockNumberProgressTracker{
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
func (t *BlockNumberProgressTracker) IncrementCompletedBlocks(amount uint64) {
	t.completedBlocks.Add(amount)
}

// LogProgress logs the progress of the migration.
func (t *BlockNumberProgressTracker) LogProgress() {
	percentage := float64(t.completedBlocks.Load()) / float64(t.totalBlocks) * 100
	elapsed := time.Since(t.startTimestamp)
	t.logger.Infow(
		"Migration progress",
		"percentage", fmt.Sprintf("%.2f%%", percentage),
		"elapsed", fmt.Sprintf("%.2fs", elapsed.Seconds()),
	)
}
