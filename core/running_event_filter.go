package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
)

// RunningEventFilterInitializer brings a lazy RunningEventFilter to a working
// state on first access.
type RunningEventFilterInitializer func(db.KeyValueStore) (*RunningEventFilter, error)

// RunningEventFilter provides a thread-safe wrapper around AggregatedBloomFilter
// that automatically manages the creation of new filters when the current one
// reaches its capacity. It maintains the current state of event filtering across
// the blockchain.
type RunningEventFilter struct {
	inner    *AggregatedBloomFilter // The current aggregated filter
	next     uint64                 // The next block number to process
	mu       sync.RWMutex
	database db.KeyValueStore

	initialize RunningEventFilterInitializer
	initErr    error
	lazyOnce   sync.Once
}

func (f *RunningEventFilter) ensureInit() error {
	if f.initialize != nil {
		f.lazyOnce.Do(func() {
			filter, err := f.initialize(f.database)
			if err != nil {
				f.initErr = fmt.Errorf("couldn't initialize the running event filter: %w", err)
				return
			}
			f.inner = filter.inner
			f.next = filter.next
		})
	}
	return f.initErr
}

// NewRunningEventFilterHot returns a RunningEventFilter that wraps the provided
// aggregated filter with the expected next block to process.
func NewRunningEventFilterHot(
	database db.KeyValueStore,
	filter *AggregatedBloomFilter,
	nextBlock uint64,
) *RunningEventFilter {
	return &RunningEventFilter{
		database: database,
		inner:    filter,
		next:     nextBlock,
	}
}

// NewRunningEventFilterLazy returns a RunningEventFilter whose state is
// initialized on first access via the supplied initializer.
func NewRunningEventFilterLazy(
	database db.KeyValueStore,
	initialize RunningEventFilterInitializer,
) *RunningEventFilter {
	return &RunningEventFilter{database: database, initialize: initialize}
}

// Insert adds a bloom filter for a single block, updating the internal
// aggregated filter. If the current window is full, it will be persisted using
// WriteAggregatedBloomFilter and a new window will be started.
// This implementation assumes blocks are not missed.
// Returns an error if the block cannot be inserted.
func (f *RunningEventFilter) Insert(
	bloom *bloom.BloomFilter,
	blockNumber uint64,
) error {
	return f.insert(f.database, bloom, blockNumber)
}

// InsertWithBatch is like [RunningEventFilter.Insert] but routes the
// window-rollover persistence through the supplied batch.
func (f *RunningEventFilter) InsertWithBatch(
	batch db.Batch,
	bloom *bloom.BloomFilter,
	blockNumber uint64,
) error {
	return f.insert(batch, bloom, blockNumber)
}

func (f *RunningEventFilter) insert(
	writer db.KeyValueWriter,
	bloom *bloom.BloomFilter,
	blockNumber uint64,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ensureInit(); err != nil {
		return err
	}

	if err := f.inner.Insert(bloom, blockNumber); err != nil {
		return fmt.Errorf(
			"inserting block %d into window [%d,%d]: %w",
			blockNumber, f.inner.FromBlock(), f.inner.ToBlock(), err,
		)
	}

	if blockNumber == f.inner.ToBlock() {
		if err := WriteAggregatedBloomFilter(writer, f.inner); err != nil {
			return fmt.Errorf(
				"persisting aggregated filter for window [%d,%d]: %w",
				f.inner.FromBlock(), f.inner.ToBlock(), err,
			)
		}
		filter := NewAggregatedFilter(blockNumber + 1)
		f.inner = &filter
	}

	f.next = blockNumber + 1
	return nil
}

// BlocksForKeys returns a bitset indicating which blocks within the range might contain
// the given keys. If no keys are provided, returns a bitset with all bits set.
func (f *RunningEventFilter) BlocksForKeys(keys [][]byte) *bitset.BitSet {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(err)
	}

	return f.inner.BlocksForKeys(keys)
}

// BlocksForKeysInto reuses a preallocated bitset (should be NumBlocksPerFilter bits).
func (f *RunningEventFilter) BlocksForKeysInto(keys [][]byte, out *bitset.BitSet) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		return err
	}

	return f.inner.BlocksForKeysInto(keys, out)
}

// FromBlock returns the starting block number of the current filter window.
func (f *RunningEventFilter) FromBlock() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(err)
	}

	return f.inner.fromBlock
}

// ToBlock returns the ending block number of the current filter window.
func (f *RunningEventFilter) ToBlock() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(err)
	}

	return f.inner.toBlock
}

// NextBlock returns the next block number to be processed.
func (f *RunningEventFilter) NextBlock() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(err)
	}

	return f.next
}

// Clone returns a deep copy of the RunningEventFilter—including a full copy of its
// internal AggregatedBloomFilter window.
func (f *RunningEventFilter) Clone() *RunningEventFilter {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(err)
	}

	innerCopy := f.inner.Clone()
	return NewRunningEventFilterHot(f.database, &innerCopy, f.next)
}

// InnerFilter returns the current AggregatedBloomFilter window.
// The returned pointer aliases internal state; mutations are visible to f.
// Use [RunningEventFilter.Clone] for an independent copy.
func (f *RunningEventFilter) InnerFilter() *AggregatedBloomFilter {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(err)
	}

	return f.inner
}

// OnReorg reverts the last processed block from the running filter. Writes
// the persisted-filter deletion on backward boundary cross directly to
// f.database; use [RunningEventFilter.OnReorgWithBatch] to commit it
// atomically with the caller's revert batch.
func (f *RunningEventFilter) OnReorg() error {
	return f.onReorg(f.database)
}

// OnReorgWithBatch is like [RunningEventFilter.OnReorg] but routes the
// stale-filter deletion through the supplied batch, so it commits or
// rolls back atomically with the caller's chain-state revert.
func (f *RunningEventFilter) OnReorgWithBatch(batch db.Batch) error {
	return f.onReorg(batch)
}

func (f *RunningEventFilter) onReorg(writer db.KeyValueWriter) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ensureInit(); err != nil {
		return err
	}

	currRangeStart := f.inner.FromBlock()
	curBlock := f.next - 1
	// Falls into previous filter's range
	if curBlock == currRangeStart-1 {
		// Drop the persisted filter; in-memory clears are about to be discarded
		// on swap and a future rollover will repopulate this window.
		if err := DeleteAggregatedBloomFilter(
			writer, f.inner.FromBlock(), f.inner.ToBlock(),
		); err != nil {
			return fmt.Errorf(
				"deleting stale persisted filter for window [%d,%d]: %w",
				f.inner.FromBlock(), f.inner.ToBlock(), err,
			)
		}

		rangeStartAligned := curBlock - (curBlock % NumBlocksPerFilter)
		rangeEndAligned := rangeStartAligned + NumBlocksPerFilter - 1

		lastStoredFilter, err := GetAggregatedBloomFilter(
			f.database,
			rangeStartAligned,
			rangeEndAligned,
		)
		if err != nil {
			return err
		}
		f.inner = &lastStoredFilter
	}

	f.next = curBlock
	return f.inner.clear(curBlock)
}

// Write writes the current state of the RunningEventFilter to persistent storage.
func (f *RunningEventFilter) Write() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ensureInit(); err != nil {
		return err
	}

	return WriteRunningEventFilter(f.database, f)
}

// InitializeRunningEventFilter is the default initializer for nodes that do
// not prune. It assumes every block below the chain head is fully retained;
// the rebuild walks back to genesis if needed and errors out when a header is
// missing. For pruning nodes, use [pruner.InitializeRunningEventFilter] instead.
func InitializeRunningEventFilter(database db.KeyValueStore) (*RunningEventFilter, error) {
	latest, err := GetChainHeight(database)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			filter := NewAggregatedFilter(0)
			return NewRunningEventFilterHot(database, &filter, 0), nil
		}
		return nil, fmt.Errorf("getting chain height: %w", err)
	}

	stored, err := GetRunningEventFilter(database)
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return nil, fmt.Errorf("getting stored running event filter: %w", err)
	}
	if err == nil {
		next := stored.NextBlock()
		// Caught up — return the snapshot as-is.
		if next == latest+1 {
			return NewRunningEventFilterHot(database, stored.InnerFilter(), next), nil
		}
		// Same-window gap — resume the snapshot and fill in place.
		if next <= latest && latest <= stored.InnerFilter().ToBlock() {
			rf := NewRunningEventFilterHot(database, stored.InnerFilter(), next)
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
	rf, err := rebuildRunningEventFilter(database, latest)
	if err != nil {
		return nil, fmt.Errorf("rebuilding running event filter up to %d: %w", latest, err)
	}
	return rf, nil
}

// fillRunningEventFilter walks [from, latest] and Inserts each block's
// EventsBloom into rf. Errors if any header in the range is missing.
func fillRunningEventFilter(
	database db.KeyValueStore,
	rf *RunningEventFilter,
	from,
	latest uint64,
) error {
	for blockNum := from; blockNum <= latest; blockNum++ {
		header, err := GetBlockHeaderByNumber(database, blockNum)
		if err != nil {
			return fmt.Errorf("getting block header by number %d: %w", blockNum, err)
		}
		if err := rf.Insert(header.EventsBloom, blockNum); err != nil {
			return fmt.Errorf("inserting block %d in events bloom: %w", blockNum, err)
		}
	}
	return nil
}

// rebuildRunningEventFilter walks back from latest to find the most recent
// persisted aggregated filter, then fills forward to latest by inserting each
// block's bloom. Used by [InitializeRunningEventFilter] on non-pruning nodes —
// every header in the fill range is expected to be present.
func rebuildRunningEventFilter(
	database db.KeyValueStore,
	latest uint64,
) (*RunningEventFilter, error) {
	rangeStartAligned := latest - latest%NumBlocksPerFilter
	lastStoredFilterRangeEnd := rangeStartAligned + NumBlocksPerFilter - 1

	var continueFrom uint64
	for {
		_, err := GetAggregatedBloomFilter(database, rangeStartAligned, lastStoredFilterRangeEnd)
		if err == nil {
			continueFrom = lastStoredFilterRangeEnd + 1
			break
		}
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, fmt.Errorf(
				"scanning for aggregated bloom filter at range [%d, %d]: %w",
				rangeStartAligned, lastStoredFilterRangeEnd, err)
		}
		if rangeStartAligned == 0 {
			break
		}
		rangeStartAligned -= NumBlocksPerFilter
		lastStoredFilterRangeEnd -= NumBlocksPerFilter
	}

	filter := NewAggregatedFilter(continueFrom)
	runningFilter := NewRunningEventFilterHot(database, &filter, continueFrom)
	if err := fillRunningEventFilter(database, runningFilter, continueFrom, latest); err != nil {
		return nil, fmt.Errorf(
			"filling running event filter [%d, %d]: %w",
			continueFrom, latest, err,
		)
	}
	return runningFilter, nil
}

func (f *RunningEventFilter) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer

	// Marshal the AggregatedBloomFilter (inner)
	innerBytes, err := f.inner.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal AggregatedBloomFilter: %w", err)
	}

	if err := binary.Write(&buf, binary.BigEndian, uint32(len(innerBytes))); err != nil {
		return nil, fmt.Errorf("write inner filter length: %w", err)
	}
	if _, err := buf.Write(innerBytes); err != nil {
		return nil, fmt.Errorf("write inner filter bytes: %w", err)
	}

	if err := binary.Write(&buf, binary.BigEndian, f.next); err != nil {
		return nil, fmt.Errorf("write next block: %w", err)
	}

	return buf.Bytes(), nil
}

func (f *RunningEventFilter) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)

	var innerLen uint32
	if err := binary.Read(r, binary.BigEndian, &innerLen); err != nil {
		return fmt.Errorf("read inner filter length: %w", err)
	}
	innerBytes := make([]byte, innerLen)
	if _, err := io.ReadFull(r, innerBytes); err != nil {
		return fmt.Errorf("read inner filter bytes: %w", err)
	}

	f.inner = &AggregatedBloomFilter{}
	if err := f.inner.UnmarshalBinary(innerBytes); err != nil {
		return fmt.Errorf("unmarshal AggregatedBloomFilter: %w", err)
	}

	if err := binary.Read(r, binary.BigEndian, &f.next); err != nil {
		return fmt.Errorf("read next block: %w", err)
	}

	f.initErr = nil
	f.mu = sync.RWMutex{}
	f.lazyOnce = sync.Once{}
	f.initialize = nil

	return nil
}
