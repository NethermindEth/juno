package core

import (
	"errors"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
)

// RunningEventFilter provides a thread-safe wrapper around AggregatedBloomFilter
// that automatically manages the creation of new filters when the current one
// reaches its capacity. It maintains the current state of event filtering across
// the blockchain.
type RunningEventFilter struct {
	inner *AggregatedBloomFilter // The current aggregated filter
	next  uint64                 // The next block number to process
	mu    sync.RWMutex
}

// NewRunningFilter returns a RunningEventFilter that wraps the provided aggregated filter
// with the expected next block to process.
func NewRunningFilter(filter *AggregatedBloomFilter, nextBlock uint64) *RunningEventFilter {
	return &RunningEventFilter{
		inner: filter,
		next:  nextBlock,
	}
}

// Insert adds a bloom filter for a single block, updating the internal
// aggregated filter. If the current window is full, it will be persisted using
// WriteAggregatedBloomFilter and a new window will be started.
// This implementation assumes blocks are not missed.
// Returns an error if the block cannot be inserted.
func (f *RunningEventFilter) Insert(
	w db.KeyValueWriter,
	bloom *bloom.BloomFilter,
	blockNumber uint64,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := f.inner.Insert(bloom, blockNumber)
	if err != nil {
		return err
	}

	f.next = blockNumber + 1

	if blockNumber == f.inner.ToBlock() {
		err := WriteAggregatedBloomFilter(w, f.inner)
		if err != nil {
			return err
		}
		f.inner = NewAggregatedFilter(blockNumber + 1)
		f.next = blockNumber + 1
	}

	return nil
}

// BlocksForKeys returns a bitset indicating which blocks within the range might contain
// the given keys. If no keys are provided, returns a bitset with all bits set.
func (f *RunningEventFilter) BlocksForKeys(keys [][]byte) *bitset.BitSet {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.inner.BlocksForKeys(keys)
}

// FromBlock returns the starting block number of the current filter window.
func (f *RunningEventFilter) FromBlock() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.inner.fromBlock
}

// ToBlock returns the ending block number of the current filter window.
func (f *RunningEventFilter) ToBlock() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.inner.toBlock
}

// NextBlock returns the next block number to be processed.
func (f *RunningEventFilter) NextBlock() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.next
}

// Clone returns a deep copy of the RunningEventFilter—including a full copy of its
// internal AggregatedBloomFilter window.
func (f *RunningEventFilter) Clone() *RunningEventFilter {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return NewRunningFilter(f.inner.Copy(), f.next)
}

// CloneInnerFilter returns a deep copy of the current AggregatedBloomFilter window.
func (f *RunningEventFilter) CloneInnerFilter() *AggregatedBloomFilter {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.inner.Copy()
}

// Clear erases the bloom filter data for the specified block in the current filter window
func (f *RunningEventFilter) Clear(blockNumber uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.inner.clear(blockNumber)
}

// Persist writes the current state of the RunningEventFilter to persistent storage.
func (f *RunningEventFilter) Persist(w db.KeyValueWriter) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return WriteRunningEventFilter(w, f)
}

// LoadRunningEventFilter attempts to load a RunningEventFilter from storage.
// If no filter is found, a new one is created from genesis. If the stored filter
// is not up to date with the chain's latest block, the filter is rebuilt.
func LoadRunningEventFilter(txn db.KeyValueStore) (*RunningEventFilter, error) {
	latest, err := GetChainHeight(txn)
	if err != nil {
		// Start from genesis
		if errors.Is(err, db.ErrKeyNotFound) {
			filter := NewAggregatedFilter(0)
			return NewRunningFilter(
				filter,
				0,
			), nil
		}
		return nil, err
	}

	filter, err := GetRunningEventFilter(txn)
	if err != nil {
		return nil, err
	}

	if filter.next == latest+1 {
		return filter, nil
	}

	return rebuildRunningEventFilter(txn, latest)
}

// rebuildRunningEventFilter constructs a RunningEventFilter state beginning from the
// latest stored filter window and sequentially adds missing block filters up to the
// blockchain's current height.
func rebuildRunningEventFilter(txn db.KeyValueStore, latest uint64) (*RunningEventFilter, error) {
	rangeStartAlligned := int64(latest - (latest % AggregateBloomBlockRangeLen))
	lastStoredFilterRangeEnd := latest - (latest % AggregateBloomBlockRangeLen) + AggregateBloomBlockRangeLen - 1

	for rangeStartAlligned >= 0 {
		_, err := GetAggregatedBloomFilter(txn, uint64(rangeStartAlligned), lastStoredFilterRangeEnd)
		if err == nil {
			break
		}

		/// Check previous range
		if errors.Is(err, db.ErrKeyNotFound) {
			rangeStartAlligned -= int64(AggregateBloomBlockRangeLen)
			lastStoredFilterRangeEnd -= AggregateBloomBlockRangeLen
			continue
		}

		return nil, err
	}

	continueFrom := lastStoredFilterRangeEnd + 1

	runningFilter := NewRunningFilter(NewAggregatedFilter(continueFrom), continueFrom)
	if lastStoredFilterRangeEnd == latest {
		return runningFilter, nil
	}

	for ; continueFrom <= latest; continueFrom++ {
		var header *Header
		header, err := GetBlockHeaderByNumber(txn, continueFrom)
		if err != nil {
			return nil, err
		}

		err = runningFilter.Insert(txn, header.EventsBloom, continueFrom)
		if err != nil {
			return nil, err
		}
	}

	return runningFilter, nil
}
