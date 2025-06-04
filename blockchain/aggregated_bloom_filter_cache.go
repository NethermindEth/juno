package blockchain

import (
	"errors"
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/bits-and-blooms/bitset"
	"github.com/ethereum/go-ethereum/common/lru"
)

// Provides cache-accelerated lookup of blockchain events
// across block ranges by aggregating bloom filters. It includes LRU-cached filters
// and efficient block iterators for event queries.

// EventFiltersCacheKey uniquely identifies a range of blocks whose aggregated bloom
// filter is cached. Used as the lookup key for bloom filter caches.
type EventFiltersCacheKey struct {
	fromBlock uint64
	toBlock   uint64
}

// AggregatedBloomFilterCache stores and manages LRU-cached aggregated bloom filters
// for block ranges, supporting fallback loading and bulk insertion.
// It is safe for concurrent use.
type AggregatedBloomFilterCache struct {
	cache        lru.Cache[EventFiltersCacheKey, *core.AggregatedBloomFilter]
	fallbackFunc func(EventFiltersCacheKey) (*core.AggregatedBloomFilter, error)
	mu           sync.Mutex
}

// NewAggregatedBloomCache creates a new LRU cache for aggregated bloom filters
// with the specified maximum size (number of ranges to cache).
func NewAggregatedBloomCache(size int) AggregatedBloomFilterCache {
	return AggregatedBloomFilterCache{
		cache: *lru.NewCache[EventFiltersCacheKey, *core.AggregatedBloomFilter](size),
	}
}

// WithFallback sets a fallback fetch function to be used if a requested
// AggregatedBloomFilter is not found in the cache. The provided function must
// return a filter matching the queried range, or an error.
func (c *AggregatedBloomFilterCache) WithFallback(fallback func(EventFiltersCacheKey) (*core.AggregatedBloomFilter, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.fallbackFunc = fallback
}

// Reset clears the entire bloom filter cache, removing all stored filters.
func (c *AggregatedBloomFilterCache) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache.Purge()
}

// SetMany inserts multiple aggregated bloom filters into the cache.
// Each filter is keyed by its block range.
func (c *AggregatedBloomFilterCache) SetMany(filters []*core.AggregatedBloomFilter) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, filter := range filters {
		c.cache.Add(EventFiltersCacheKey{fromBlock: filter.FromBlock(), toBlock: filter.ToBlock()}, filter)
	}
}

// MatchedBlockIterator iterates over candidate block numbers within a block range
// that may match an event query, using cached (or fetched) aggregated bloom filters
// for efficient windowed scanning and filtering.
type MatchedBlockIterator struct {
	currentBits        *bitset.BitSet // Current candidate blocks bitset to iterate
	nextIndex          uint64         // Next bit index to test and possibly yield
	rangeStart         uint64         // starting block number of the filter range
	currentWindowStart uint64         // absolute block start of currently loaded window
	rangeEnd           uint64         // total number of blocks in filter range
	done               bool           // iteration finished flag

	maxScanned   uint // max number of blocks to iterate (0 = unlimited)
	scannedCount uint // number of blocks yielded so far

	cache         *AggregatedBloomFilterCache
	runningFilter *core.RunningEventFilter
	matcher       *EventMatcher
}

var (
	ErrInvalidBlockRange                = errors.New("fromBlock > toBlock")
	ErrMaxScannedBlockLimitExceed       = errors.New("max scanned blocks exceeded")
	ErrAggregatedBloomFilterFallbackNil = errors.New("aggregated bloom filter does not have fallback")
	ErrNilRunningFilter                 = errors.New("running filter is nil")
	ErrFetchedFilterBoundsMismatch      = errors.New("fetched filter bounds mismatch")
)

// NewMatchedBlockIterator constructs an iterator for block numbers within [fromBlock, toBlock]
// that may match the given EventMatcher. The scan can be limited to maxScanned candidate
// blocks. It uses cached (or fetched via fallback) AggregatedBloomFilter windows for
// efficiency.
// Returns an error if input is invalid or required state is missing.
func (c *AggregatedBloomFilterCache) NewMatchedBlockIterator(
	fromBlock, toBlock uint64,
	maxScanned uint,
	matcher *EventMatcher,
	runningFilter *core.RunningEventFilter,
) (*MatchedBlockIterator, error) {
	if fromBlock > toBlock {
		return nil, ErrInvalidBlockRange
	}

	if runningFilter == nil {
		return nil, ErrNilRunningFilter
	}

	windowStart := fromBlock - (fromBlock % core.AggregateBloomBlockRangeLen)
	return &MatchedBlockIterator{
		rangeStart:         fromBlock,
		rangeEnd:           toBlock,
		maxScanned:         maxScanned,
		cache:              c,
		runningFilter:      runningFilter,
		matcher:            matcher,
		currentWindowStart: windowStart,
	}, nil
}

// loadNextWindow prepares the iterator to scan the next window of blocks,
// loading or fetching the corresponding AggregatedBloomFilter as necessary.
// Advances currentBits and nextIndex appropriately for iteration.
// Returns an error if the cache or fallback retrieval fails, or if a filter's block range is inconsistent.
func (it *MatchedBlockIterator) loadNextWindow() error {
	if it.done {
		return nil
	}

	// Calculate next window start aligned to block range
	var windowStart uint64
	if it.currentBits == nil {
		windowStart = it.currentWindowStart
		it.nextIndex = it.rangeStart % core.AggregateBloomBlockRangeLen // offset for first window
	} else {
		windowStart = it.currentWindowStart + core.AggregateBloomBlockRangeLen
		it.nextIndex = 0 // offset 0 for subsequent windows
	}

	if windowStart > it.rangeEnd {
		it.done = true
		return nil
	}
	fromAligned := windowStart - (windowStart % core.AggregateBloomBlockRangeLen)
	toAligned := fromAligned + core.AggregateBloomBlockRangeLen - 1

	key := EventFiltersCacheKey{fromBlock: fromAligned, toBlock: toAligned}
	filter, ok := it.cache.cache.Get(key)

	if ok {
		it.currentBits = it.matcher.getCandidateBlocksForFilter(filter)
		it.currentWindowStart = fromAligned // set current window start absolute index
		return nil
	}

	// Not found in cache and fall into range of running filter
	if fromAligned == it.runningFilter.FromBlock() {
		it.currentBits = it.matcher.getCandidateBlocksForFilter(it.runningFilter.InnerFilter())
		it.currentWindowStart = fromAligned // set current window start absolute index
		return nil
	}

	// Not found in cache and not fall into range of running filter
	if it.cache.fallbackFunc == nil {
		return ErrAggregatedBloomFilterFallbackNil
	}

	filter, err := it.cache.fallbackFunc(key)
	if err != nil {
		return err
	}

	if filter.FromBlock() != fromAligned || filter.ToBlock() != toAligned {
		return ErrFetchedFilterBoundsMismatch
	}

	it.cache.cache.Add(EventFiltersCacheKey{fromBlock: filter.FromBlock(), toBlock: filter.ToBlock()}, filter)

	it.currentBits = it.matcher.getCandidateBlocksForFilter(filter)
	it.currentWindowStart = fromAligned // set current window start absolute index
	return nil
}

// Next advances the iterator to the next matching block number within the scanned range.
// Returns the next candidate block number (absolute), a boolean indicating if such exists,
// and any error encountered (including scan limit exhaustion or fallback fetch errors).
// When ok == false and error is nil, the iteration is complete.
func (it *MatchedBlockIterator) Next() (uint64, bool, error) {
	if it.done {
		return 0, false, nil
	}

	/// Load the first filter
	if it.currentBits == nil {
		if err := it.loadNextWindow(); err != nil {
			it.done = true
			return 0, false, err
		}
		if it.done {
			return 0, false, nil
		}
	}

	// Search till finding next set bit or iterator exhausts
	next, found := it.currentBits.NextSet(uint(it.nextIndex))
	for !found {
		if err := it.loadNextWindow(); err != nil {
			it.done = true
			return 0, false, err
		}

		if it.done {
			return 0, false, nil
		}
		next, found = it.currentBits.NextSet(uint(it.nextIndex))
	}

	// Calculate absolute block number relative to current window
	blockNum := it.currentWindowStart + uint64(next)
	if blockNum > it.rangeEnd {
		it.done = true
		return 0, false, nil
	}
	it.nextIndex = uint64(next) + 1

	if it.maxScanned > 0 {
		it.scannedCount++
		if it.scannedCount > it.maxScanned {
			it.done = true
			return blockNum, false, ErrMaxScannedBlockLimitExceed
		}
	}

	return blockNum, true, nil
}
