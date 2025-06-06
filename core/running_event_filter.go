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

// RunningEventFilter provides a thread-safe wrapper around AggregatedBloomFilter
// that automatically manages the creation of new filters when the current one
// reaches its capacity. It maintains the current state of event filtering across
// the blockchain.
type RunningEventFilter struct {
	inner *AggregatedBloomFilter // The current aggregated filter
	next  uint64                 // The next block number to process
	mu    sync.RWMutex
	txn   db.KeyValueStore

	initErr  error
	lazyOnce sync.Once
	lazyInit bool
}

func (f *RunningEventFilter) ensureInit() error {
	if f.lazyInit {
		f.lazyOnce.Do(func() {
			filter, err := loadRunningEventFilter(f.txn)
			if err != nil {
				f.initErr = err
				return
			}
			f.inner = filter.inner
			f.next = filter.next
		})
	}
	return f.initErr
}

// NewRunningFilter returns a RunningEventFilter that wraps the provided aggregated filter
// with the expected next block to process.
func NewRunningEventFilterHot(txn db.KeyValueStore, filter *AggregatedBloomFilter, nextBlock uint64) *RunningEventFilter {
	return &RunningEventFilter{
		txn:   txn,
		inner: filter,
		next:  nextBlock,
	}
}

func NewRunningEventFilterLazy(txn db.KeyValueStore) *RunningEventFilter {
	return &RunningEventFilter{txn: txn, lazyInit: true}
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
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ensureInit(); err != nil {
		return err
	}

	err := f.inner.Insert(bloom, blockNumber)
	if err != nil {
		return err
	}

	if blockNumber == f.inner.ToBlock() {
		err := WriteAggregatedBloomFilter(f.txn, f.inner)
		if err != nil {
			return err
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
		panic(fmt.Sprintf("Couldn't initialised the running event filter. Error: %v", err))
	}

	return f.inner.BlocksForKeys(keys)
}

// BlocksForKeysInto reuses a preallocated bitset (should be AggregateBloomBlockRangeLen bits).
func (f *RunningEventFilter) BlocksForKeysInto(keys [][]byte, out *bitset.BitSet) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(fmt.Sprintf("Couldn't initialised the running event filter. Error: %v", err))
	}

	return f.inner.BlocksForKeysInto(keys, out)
}

// FromBlock returns the starting block number of the current filter window.
func (f *RunningEventFilter) FromBlock() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(fmt.Sprintf("Couldn't initialised the running event filter. Error: %v", err))
	}

	return f.inner.fromBlock
}

// ToBlock returns the ending block number of the current filter window.
func (f *RunningEventFilter) ToBlock() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(fmt.Sprintf("Couldn't initialised the running event filter. Error: %v", err))
	}

	return f.inner.toBlock
}

// NextBlock returns the next block number to be processed.
func (f *RunningEventFilter) NextBlock() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(fmt.Sprintf("Couldn't initialised the running event filter. Error: %v", err))
	}

	return f.next
}

// Clone returns a deep copy of the RunningEventFilter—including a full copy of its
// internal AggregatedBloomFilter window.
func (f *RunningEventFilter) Clone() *RunningEventFilter {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(fmt.Sprintf("Couldn't initialised the running event filter. Error: %v", err))
	}

	innerCopy := f.inner.Clone()
	return NewRunningEventFilterHot(f.txn, &innerCopy, f.next)
}

// InnerFilter returns a deep copy of the current AggregatedBloomFilter window.
func (f *RunningEventFilter) InnerFilter() *AggregatedBloomFilter {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.ensureInit(); err != nil {
		panic(fmt.Sprintf("Couldn't initialised the running event filter. Error: %v", err))
	}

	return f.inner
}

// Clear erases the bloom filter data for the specified block
func (f *RunningEventFilter) OnReorg() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ensureInit(); err != nil {
		panic(fmt.Sprintf("Couldn't initialised the running event filter. Error: %v", err))
	}

	currRangeStart := f.inner.FromBlock()
	curBlock := f.next - 1
	// Falls into previous filters range
	if curBlock == currRangeStart-1 {
		rangeStartAlligned := curBlock - (curBlock % AggregateBloomBlockRangeLen)
		rangeEndAlligned := rangeStartAlligned + AggregateBloomBlockRangeLen - 1

		lastStoredFilter, err := GetAggregatedBloomFilter(f.txn, rangeStartAlligned, rangeEndAlligned)
		if err != nil {
			return err
		}
		f.inner = lastStoredFilter
	}

	f.next = curBlock
	return f.inner.clear(curBlock)
}

// Persist writes the current state of the RunningEventFilter to persistent storage.
func (f *RunningEventFilter) Persist() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ensureInit(); err != nil {
		panic(fmt.Sprintf("Couldn't initialised the running event filter. Error: %v", err))
	}

	return WriteRunningEventFilter(f.txn, f)
}

// loadRunningEventFilter attempts to load a RunningEventFilter from storage.
// If no filter is found, a new one is created from genesis. If the stored filter
// is not up to date with the chain's latest block, the filter is rebuilt.
func loadRunningEventFilter(txn db.KeyValueStore) (*RunningEventFilter, error) {
	latest, err := GetChainHeight(txn)
	if err != nil {
		// Start from genesis
		if errors.Is(err, db.ErrKeyNotFound) {
			filter := NewAggregatedFilter(0)
			return NewRunningEventFilterHot(
				txn,
				&filter,
				0,
			), nil
		}
		return nil, err
	}

	rf, err := GetRunningEventFilter(txn)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			filter := NewAggregatedFilter(0)
			// for case where node crashed and didnt gracefully persist the filter for block range  (genesis, aggregated filter range)
			rf = NewRunningEventFilterHot(
				txn,
				&filter,
				0,
			)
		} else {
			return nil, err
		}
	}
	rf.txn = txn

	if rf.next == latest+1 {
		return rf, nil
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
	filter := NewAggregatedFilter(continueFrom)
	runningFilter := NewRunningEventFilterHot(txn, &filter, continueFrom)
	if lastStoredFilterRangeEnd == latest {
		return runningFilter, nil
	}

	for ; continueFrom <= latest; continueFrom++ {
		var header *Header
		header, err := GetBlockHeaderByNumber(txn, continueFrom)
		if err != nil {
			return nil, err
		}

		err = runningFilter.Insert(header.EventsBloom, continueFrom)
		if err != nil {
			return nil, err
		}
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
	f.lazyInit = false // or true if you want to reconstruct as lazy

	return nil
}
