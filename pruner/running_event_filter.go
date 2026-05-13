package pruner

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
)

// InitializeRunningEventFilter is the pruning-aware initializer. Brings the
// running event filter up to chain head, reusing the persisted snapshot when
// possible and rebuilding otherwise. Use [core.InitializeRunningEventFilter]
// on non-pruning nodes.
func InitializeRunningEventFilter(database db.KeyValueStore) (*core.RunningEventFilter, error) {
	latest, err := core.GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			filter := core.NewAggregatedFilter(0)
			return core.NewRunningEventFilterHot(database, &filter, 0), nil
		}
		return nil, fmt.Errorf("getting chain height: %w", err)
	}

	floor, err := OldestRetainedBlock(database)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("getting oldest retained block: %w", err)
	}

	stored, err := core.GetRunningEventFilter(database)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("getting stored running event filter: %w", err)
	}
	if err == nil {
		next := stored.NextBlock()
		// Caught up — return the snapshot as-is.
		if next == latest+1 {
			return core.NewRunningEventFilterHot(database, stored.InnerFilter(), next), nil
		}
		// Same-window gap — resume the snapshot and fill in place.
		if next <= latest && latest <= stored.InnerFilter().ToBlock() {
			// Clamp to floor so we don't read pruned headers. Below-floor
			// snapshot bits remain — harmless false positives, since the
			// referenced receipts are pruned and no event query can act
			// on a hit.
			next = max(next, floor)
			rf := core.NewRunningEventFilterHot(database, stored.InnerFilter(), next)
			if fillErr := fillRunningEventFilter(database, rf, next, latest); fillErr != nil {
				return nil, fmt.Errorf(
					"filling running event filter [%d, %d]: %w",
					next, latest, fillErr,
				)
			}
			return rf, nil
		}
	}

	// Multi-window gap, future-dated snapshot, or no snapshot — rebuild.
	rf, err := rebuildRunningEventFilter(database, latest, floor)
	if err != nil {
		return nil, fmt.Errorf(
			"rebuilding running event filter up to %d (floor=%d): %w",
			latest, floor, err,
		)
	}
	return rf, nil
}

// fillRunningEventFilter walks [from, end] and Inserts each block's
// EventsBloom into rf. Callers must clamp from to the retention floor so
// the range stays within retained headers — a missing header is treated
// as an error, not silently skipped.
func fillRunningEventFilter(
	txn db.KeyValueStore,
	rf *core.RunningEventFilter,
	from,
	end uint64,
) error {
	for blockNum := from; blockNum <= end; blockNum++ {
		header, err := core.GetBlockHeaderByNumber(txn, blockNum)
		if err != nil {
			return fmt.Errorf("getting block header by number %d: %w", blockNum, err)
		}
		if err := rf.Insert(header.EventsBloom, blockNum); err != nil {
			return fmt.Errorf("inserting block %d in events bloom %w", blockNum, err)
		}
	}
	return nil
}

// rebuildRunningEventFilter walks back from latest to find the most recent
// persisted aggregated filter, then fills forward to latest. The backward
// walk is bounded by floorAligned so we don't read pruned filters. When
// no anchor is found, the window is rooted at floorAligned and filling
// starts at floor itself, leaving the pruned prefix [floorAligned, floor)
// as zero bitmap rows.
func rebuildRunningEventFilter(
	database db.KeyValueStore,
	latest,
	floor uint64,
) (*core.RunningEventFilter, error) {
	floorAligned := floor - floor%core.NumBlocksPerFilter

	rangeStartAligned := latest - latest%core.NumBlocksPerFilter
	lastStoredFilterRangeEnd := rangeStartAligned + core.NumBlocksPerFilter - 1

	continueFrom := floor
	windowStart := floorAligned
	for {
		_, err := core.GetAggregatedBloomFilter(database, rangeStartAligned, lastStoredFilterRangeEnd)
		if err == nil {
			continueFrom = lastStoredFilterRangeEnd + 1
			windowStart = continueFrom
			break
		}
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, fmt.Errorf(
				"scanning for aggregated bloom filter at range [%d, %d]: %w",
				rangeStartAligned, lastStoredFilterRangeEnd, err,
			)
		}
		if rangeStartAligned <= floorAligned {
			break
		}
		rangeStartAligned -= core.NumBlocksPerFilter
		lastStoredFilterRangeEnd -= core.NumBlocksPerFilter
	}

	filter := core.NewAggregatedFilter(windowStart)
	runningFilter := core.NewRunningEventFilterHot(database, &filter, continueFrom)
	if err := fillRunningEventFilter(database, runningFilter, continueFrom, latest); err != nil {
		return nil, fmt.Errorf(
			"filling running event filter [%d, %d] (floor=%d): %w",
			continueFrom, latest, floor, err,
		)
	}
	return runningFilter, nil
}
