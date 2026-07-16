package blockchain

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/lru"
	"github.com/bits-and-blooms/bitset"
	"golang.org/x/sync/singleflight"
)

// NOTE(Ege): consider making it configurable
const AggregatedBloomFilterCacheSize = 16

// AggregatedBloomCacheListener observes cache behaviour for metrics. All
// methods must be safe for concurrent use.
type AggregatedBloomCacheListener interface {
	// OnHit fires when a lookup is served from the LRU.
	OnHit()
	// OnMiss fires for every lookup not served from the LRU (before
	// singleflight coalescing). loads - misses gives coalesced requests.
	OnMiss()
	// OnLoad fires once per actual fallback load, reporting filter density
	// (setBits/totalBits) and the load duration.
	OnLoad(setBits, totalBits uint64, dur time.Duration)
}

// SelectiveAggregatedBloomCacheListener is a no-op-by-default listener whose
// callbacks can be set individually.
type SelectiveAggregatedBloomCacheListener struct {
	OnHitCb  func()
	OnMissCb func()
	OnLoadCb func(setBits, totalBits uint64, dur time.Duration)
}

func (l *SelectiveAggregatedBloomCacheListener) OnHit() {
	if l.OnHitCb != nil {
		l.OnHitCb()
	}
}

func (l *SelectiveAggregatedBloomCacheListener) OnMiss() {
	if l.OnMissCb != nil {
		l.OnMissCb()
	}
}

func (l *SelectiveAggregatedBloomCacheListener) OnLoad(setBits, totalBits uint64, dur time.Duration) {
	if l.OnLoadCb != nil {
		l.OnLoadCb(setBits, totalBits, dur)
	}
}

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
	cache        *lru.Cache[EventFiltersCacheKey, *core.AggregatedBloomFilter]
	fallbackFunc func(EventFiltersCacheKey) (core.AggregatedBloomFilter, error)
	// group collapses concurrent fallback loads for the same range into a
	// single DB read + decode.
	group    singleflight.Group
	listener AggregatedBloomCacheListener
}

// NewAggregatedBloomCache creates a new LRU cache for aggregated bloom filters
// with the specified maximum size (number of ranges to cache).
func NewAggregatedBloomCache(size int) *AggregatedBloomFilterCache {
	return &AggregatedBloomFilterCache{
		cache: lru.New[
			EventFiltersCacheKey,
			*core.AggregatedBloomFilter,
		](size),
		listener: &SelectiveAggregatedBloomCacheListener{},
	}
}

// WithFallback sets a fallback fetch function to be used if a requested
// AggregatedBloomFilter is not found in the cache. The provided function must
// return a filter matching the queried range, or an error.
func (c *AggregatedBloomFilterCache) WithFallback(fallback func(EventFiltersCacheKey) (core.AggregatedBloomFilter, error)) {
	c.fallbackFunc = fallback
}

// WithListener sets the metrics listener. A nil listener resets to no-op.
func (c *AggregatedBloomFilterCache) WithListener(listener AggregatedBloomCacheListener) {
	if listener == nil {
		listener = &SelectiveAggregatedBloomCacheListener{}
	}
	c.listener = listener
}

func (k EventFiltersCacheKey) singleflightKey() string {
	return strconv.FormatUint(k.fromBlock, 10) + ":" + strconv.FormatUint(k.toBlock, 10)
}

// getOrLoad returns the filter for key, loading it via the fallback on a cache
// miss. Concurrent misses for the same key are coalesced into one load; the
// resulting filter is read-only and safe to share across callers.
func (c *AggregatedBloomFilterCache) getOrLoad(key EventFiltersCacheKey) (*core.AggregatedBloomFilter, error) {
	if filter, ok := c.cache.Get(key); ok {
		c.listener.OnHit()
		return filter, nil
	}

	c.listener.OnMiss()
	if c.fallbackFunc == nil {
		return nil, ErrAggregatedBloomFilterFallbackNil
	}

	filter, err, _ := c.group.Do(key.singleflightKey(), func() (any, error) {
		// A concurrent flight may have populated the cache between our miss
		// and acquiring the flight; re-check before hitting the DB.
		if filter, ok := c.cache.Get(key); ok {
			return filter, nil
		}

		start := time.Now()
		fetched, err := c.fallbackFunc(key)
		if err != nil {
			return nil, fmt.Errorf("fetching aggregated bloom filter via fallback: %w", err)
		}
		if fetched.FromBlock() != key.fromBlock || fetched.ToBlock() != key.toBlock {
			return nil, ErrFetchedFilterBoundsMismatch
		}

		filter := &fetched
		c.cache.Add(key, filter)
		c.listener.OnLoad(filter.SetBitCount(), filter.TotalBitCount(), time.Since(start))
		return filter, nil
	})
	if err != nil {
		return nil, err
	}
	return filter.(*core.AggregatedBloomFilter), nil
}

// Reset clears the entire bloom filter cache, removing all stored filters.
func (c *AggregatedBloomFilterCache) Reset() {
	c.cache.Purge()
}

// SetMany inserts multiple aggregated bloom filters into the cache.
// Each filter is keyed by its block range.
func (c *AggregatedBloomFilterCache) SetMany(filters []*core.AggregatedBloomFilter) {
	for _, filter := range filters {
		c.cache.Add(
			EventFiltersCacheKey{
				fromBlock: filter.FromBlock(),
				toBlock:   filter.ToBlock(),
			},
			filter,
		)
	}
}

// MatchedBlockIterator iterates over candidate block numbers within a block range
// that may match an event query, using cached (or fetched) aggregated bloom filters
// for efficient windowed scanning and filtering.
type MatchedBlockIterator struct {
	currentBits        *bitset.BitSet // current candidate blocks bitset to iterate
	nextIndex          uint64         // next bit index to test and possibly yield
	rangeStart         uint64         // starting block number of the filter range
	currentWindowStart uint64         // absolute block start of currently loaded window
	rangeEnd           uint64         // end block number of the filter range
	done               bool           // iteration finished flag

	maxScanned   uint64 // max number of blocks to iterate (0 = unlimited)
	scannedCount uint64 // number of blocks yielded so far

	cache         *AggregatedBloomFilterCache
	runningFilter *core.RunningEventFilter
	matcher       *EventMatcher
}

var (
	ErrMaxScannedBlockLimitExceed       = errors.New("max scanned blocks exceeded")
	ErrAggregatedBloomFilterFallbackNil = errors.New("aggregated bloom filter does not have fallback")
	ErrFetchedFilterBoundsMismatch      = errors.New("fetched filter bounds mismatch")
	ErrNilRunningFilter                 = errors.New("running filter is nil")
)

// NewMatchedBlockIterator constructs an iterator for block numbers within [fromBlock, toBlock]
// that may match the given EventMatcher. The scan can be limited to maxScanned candidate
// blocks. It uses cached (or fetched via fallback) AggregatedBloomFilter windows for
// efficiency.
// Returns an error if input is invalid or required state is missing.
func (c *AggregatedBloomFilterCache) NewMatchedBlockIterator(
	fromBlock, toBlock uint64,
	maxScanned uint64,
	matcher *EventMatcher,
	runningFilter *core.RunningEventFilter,
) (MatchedBlockIterator, error) {
	if runningFilter == nil {
		return MatchedBlockIterator{}, ErrNilRunningFilter
	}

	windowStart := fromBlock - (fromBlock % core.NumBlocksPerFilter)
	return MatchedBlockIterator{
		rangeStart:         fromBlock,
		rangeEnd:           toBlock,
		maxScanned:         maxScanned,
		cache:              c,
		runningFilter:      runningFilter,
		matcher:            matcher,
		currentWindowStart: windowStart,
		// If from_block > to_block return exhausted iterator
		done: fromBlock > toBlock,
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
		it.currentBits = bitset.New(uint(core.NumBlocksPerFilter))
		windowStart = it.currentWindowStart
		it.nextIndex = it.rangeStart % core.NumBlocksPerFilter // offset for first window
	} else {
		windowStart = it.currentWindowStart + core.NumBlocksPerFilter
		it.nextIndex = 0 // offset 0 for subsequent windows
	}

	if windowStart > it.rangeEnd {
		it.done = true
		return nil
	}

	fromAligned := windowStart - (windowStart % core.NumBlocksPerFilter)
	toAligned := fromAligned + core.NumBlocksPerFilter - 1

	// Falls into range of running filter
	runningFrom, err := it.runningFilter.FromBlock()
	if err != nil {
		return fmt.Errorf("reading running filter from-block: %w", err)
	}
	if fromAligned == runningFrom {
		inner, err := it.runningFilter.InnerFilter()
		if err != nil {
			return fmt.Errorf("reading running filter inner filter: %w", err)
		}
		err = it.matcher.getCandidateBlocksForFilterInto(inner, it.currentBits)
		if err != nil {
			return fmt.Errorf("getting candidate blocks for running filter: %w", err)
		}
		it.currentWindowStart = fromAligned // set current window start absolute index
		return nil
	}

	key := EventFiltersCacheKey{fromBlock: fromAligned, toBlock: toAligned}
	filter, err := it.cache.getOrLoad(key)
	if err != nil {
		return err
	}

	if err := it.matcher.getCandidateBlocksForFilterInto(filter, it.currentBits); err != nil {
		return fmt.Errorf("getting candidate blocks for filter: %w", err)
	}
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
