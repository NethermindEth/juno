package blockchain_test

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

// randomContractAddress generates a random contract address (felt) for testing.
func randomContractAddress(t *testing.T) *felt.Felt {
	t.Helper()
	addr := felt.NewRandom[felt.Felt]()
	return addr
}

// randomEventKeys generates a random 2D slice of felts to simulate event log keys.
// keysPerEvent sets "topic count"; subKeysPerKey is for composite keys (set to 1 for regular).
func randomEventKeys(t *testing.T, keysPerEvent, subKeysPerKey int) [][]felt.Felt {
	t.Helper()
	keys := make([][]felt.Felt, keysPerEvent)
	for i := range keys {
		keys[i] = make([]felt.Felt, subKeysPerKey)
		for j := range keys[i] {
			key := felt.NewRandom[felt.Felt]()
			keys[i][j] = *key
		}
	}
	return keys
}

// EventTestCase describes an event/contract with its block mappings for assertions.
type eventTestCase struct {
	contractAddress *felt.Felt
	keys            [][]felt.Felt
	expectedBlocks  []uint64 // Populated by the insert routine
}

// GenerateRandomEvents creates n random events (addresses+keys) for use in filters.
func generateRandomEvents(t *testing.T, n, keysPerEvent, subKeysPerKey int) []*eventTestCase {
	t.Helper()
	events := make([]*eventTestCase, n)
	for i := range n {
		addr := randomContractAddress(t)
		keys := randomEventKeys(t, keysPerEvent, subKeysPerKey)
		events[i] = &eventTestCase{
			contractAddress: addr,
			keys:            keys,
			expectedBlocks:  []uint64{},
		}
	}
	return events
}

// PopulateAggregatedBloomFilters generates AggregatedBloomFilters for a block segment,
// fills them with random events, and tracks which events went in which block.
func populateAggregatedBloomFilters(
	t *testing.T,
	numAggregatedFilters uint64,
	events []*eventTestCase,
	blocksPerFilter uint64,
) []*core.AggregatedBloomFilter {
	t.Helper()
	filterSet := make([]*core.AggregatedBloomFilter, numAggregatedFilters)
	fromBlock := uint64(0)
	for i := range numAggregatedFilters {
		filter := core.NewAggregatedFilter(fromBlock)
		for j := range blocksPerFilter {
			blockNumber := fromBlock + j
			bloomInst := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
			insertedEvents := make(map[int]struct{})
			// For this test, random events in each block
			numEvents := 1 + rand.Intn(3)
			for range numEvents {
				eventIdx := rand.Intn(len(events))
				if _, already := insertedEvents[eventIdx]; already {
					continue // avoid double-inserting same event in this block
				}
				insertedEvents[eventIdx] = struct{}{}
				ev := events[eventIdx]
				if ev.contractAddress != nil {
					bloomInst.Add(ev.contractAddress.Marshal())
				}
				// Insert all keys for the event
				for index, keySet := range ev.keys {
					for _, key := range keySet {
						keyBytes := key.Bytes()
						keyAndIndexBytes := binary.AppendVarint(keyBytes[:], int64(index))
						bloomInst.Add(keyAndIndexBytes)
					}
				}
				// track for assertion
				events[eventIdx].expectedBlocks = append(events[eventIdx].expectedBlocks, blockNumber)
			}
			require.NoError(t, filter.Insert(bloomInst, blockNumber))
		}
		filterSet[i] = &filter
		fromBlock += blocksPerFilter
	}
	return filterSet
}

// PopulateAggregatedBloomFilters generates AggregatedBloomFilters for a block segment,
// fills them deterministically by emitted given event every nth block
func populateAggregatedBloomDeterministic(
	t *testing.T,
	numAggregatedFilters uint64,
	test *eventTestCase,
	blocksPerFilter uint64,
	emmittedEvery uint64,
) []*core.AggregatedBloomFilter {
	t.Helper()
	// Setup aggregated bloom filters per block range

	filters := make([]*core.AggregatedBloomFilter, numAggregatedFilters)
	fromBlock := uint64(0)

	for i := range numAggregatedFilters {
		filter := core.NewAggregatedFilter(fromBlock)

		for j := range blocksPerFilter {
			blockNumber := fromBlock + j

			// only emit this event once per 4 block
			if blockNumber%emmittedEvery != 0 {
				continue
			}
			innerBloom := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)

			if test.contractAddress != nil {
				innerBloom.Add(test.contractAddress.Marshal())
			}
			// Flatten keys from all positions in event filter keys
			for index, keySet := range test.keys {
				for _, key := range keySet {
					keyBytes := key.Bytes()
					keyAndIndexBytes := binary.AppendVarint(keyBytes[:], int64(index))
					innerBloom.Add(keyAndIndexBytes)
				}
			}
			test.expectedBlocks = append(test.expectedBlocks, blockNumber)

			require.NoError(t, filter.Insert(innerBloom, blockNumber))
		}
		// Insert keys belonging to event filters that map to this block
		filters[i] = &filter
		fromBlock += core.NumBlocksPerFilter
	}

	return filters
}

func TestMatchBlockIterator_InsertAndQueryRandomEvents(t *testing.T) {
	numEvents := 64
	numAggregatedBloomFilters := uint64(16)
	blocksPerFilter := core.NumBlocksPerFilter
	chainHeight := numAggregatedBloomFilters*blocksPerFilter - 1

	// Generate 'numEvents' random event definitions
	events := generateRandomEvents(t, numEvents, 3, 1) // 3 topics per event, no subkeys

	// Build windowed filters and populate with random event emissions
	filters := populateAggregatedBloomFilters(t, numAggregatedBloomFilters, events, blocksPerFilter)

	testDB := memory.New()
	// Create cache and insert filters
	cache := blockchain.NewAggregatedBloomCache(int(numAggregatedBloomFilters))
	cache.SetMany(filters)
	runningFilterStart := numAggregatedBloomFilters * blocksPerFilter
	innerFilter := core.NewAggregatedFilter(runningFilterStart)
	runningFilter := core.NewRunningEventFilterHot(testDB, &innerFilter, runningFilterStart)
	for _, test := range events {
		// Create iterator for event
		contractAddresses := []felt.Felt{*test.contractAddress}
		matcher := blockchain.NewEventMatcher(contractAddresses, test.keys)

		iterator, err := cache.NewMatchedBlockIterator(0, chainHeight, 0, &matcher, runningFilter)
		require.NoError(t, err)

		/// checks found blocks in range match expected ones
		for _, expectedBlock := range test.expectedBlocks {
			blockNumber, ok, err := iterator.Next()
			require.NoError(t, err)
			require.True(t, ok, "Iterator exhausted earlier than expected")
			require.Equal(t, expectedBlock, blockNumber)
		}

		blockNumber, ok, err := iterator.Next()
		require.NoError(t, err)
		require.Equal(t, uint64(0), blockNumber)
		require.False(t, ok)
	}
}

func TestMatchedBlockIterator_BasicCases(t *testing.T) {
	var numAggregatedBloomFilters uint64 = 16
	chainHeight := numAggregatedBloomFilters*core.NumBlocksPerFilter - 1

	events := generateRandomEvents(t, 1, 3, 1)
	test := events[0]
	emmitedEvery := 4
	filters := populateAggregatedBloomDeterministic(t, numAggregatedBloomFilters, test, core.NumBlocksPerFilter, uint64(emmitedEvery))

	cache := blockchain.NewAggregatedBloomCache(int(numAggregatedBloomFilters))
	cache.SetMany(filters)

	testDB := memory.New()
	var maxScannedLimit uint64 = 0
	contractAddresses := []felt.Felt{*test.contractAddress}
	eventMatcher := blockchain.NewEventMatcher(contractAddresses, test.keys)
	innerFilter := core.NewAggregatedFilter(chainHeight + 1)
	runningFilter := core.NewRunningEventFilterHot(testDB, &innerFilter, chainHeight+1)
	t.Run("returns only what is in range", func(t *testing.T) {
		var start, end, blockRange uint64 = 0, core.NumBlocksPerFilter * numAggregatedBloomFilters, core.NumBlocksPerFilter / 4

		for start < end {
			currRangeEnd := min(start+blockRange-1, numAggregatedBloomFilters*core.NumBlocksPerFilter-1)

			// Create filter for event
			iterator, err := cache.NewMatchedBlockIterator(
				start,
				currRangeEnd,
				maxScannedLimit,
				&eventMatcher,
				runningFilter,
			)
			require.NoError(t, err)

			// Round to next multiple of 4 or keep as is if already multiple of 4
			nextExpected := ((start + 3) / 4) * 4
			for nextExpected <= currRangeEnd {
				blockNumber, ok, err := iterator.Next()
				require.True(t, ok, "Iterator exhausted earlier than expected")
				require.NoError(t, err)
				require.Equal(t, nextExpected, blockNumber)
				nextExpected += 4
			}

			blockNumber, ok, err := iterator.Next()
			require.NoError(t, err)
			require.Equal(t, uint64(0), blockNumber)
			require.False(t, ok)
			start += blockRange
		}
	})
	t.Run("maxScanned stops early", func(t *testing.T) {
		maxScannedLimit = uint64(10_000)
		// Create filter for event
		iterator, err := cache.NewMatchedBlockIterator(0, chainHeight, maxScannedLimit, &eventMatcher, runningFilter)
		require.NoError(t, err)

		// for 10_000 blocks scan we need
		for range maxScannedLimit {
			_, ok, err := iterator.Next()
			require.NoError(t, err)
			require.True(t, ok, "iterator exhausted earlier than expected")
		}
		_, ok, err := iterator.Next()
		require.False(t, ok, "iterator not exhausted once exceeded max scan")
		require.ErrorIs(t, err, blockchain.ErrMaxScannedBlockLimitExceed)
	})

	t.Run("range with no matches", func(t *testing.T) {
		// Create filter for event
		iterator, err := cache.NewMatchedBlockIterator(1, 3, maxScannedLimit, &eventMatcher, runningFilter)
		require.NoError(t, err)

		_, ok, err := iterator.Next()
		require.False(t, ok)
		require.NoError(t, err)
	})

	t.Run("fromBlock > toBlock should create exhausted iterator", func(t *testing.T) {
		// FromBlock must lte to toBlock
		iter, err := cache.NewMatchedBlockIterator(2, 1, maxScannedLimit, &eventMatcher, runningFilter)
		require.NoError(t, err)
		blockNum, ok, err := iter.Next()
		require.NoError(t, err)
		require.False(t, ok)
		require.Equal(t, uint64(0), blockNum)
	})

	t.Run("range falls into running filter", func(t *testing.T) {
		currBlockFilter := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
		currBlockFilter.Add(test.contractAddress.Marshal())

		// Insert all keys for the event
		for index, keySet := range test.keys {
			for _, key := range keySet {
				keyBytes := key.Bytes()
				keyAndIndexBytes := binary.AppendVarint(keyBytes[:], int64(index))
				currBlockFilter.Add(keyAndIndexBytes)
			}
		}
		require.NoError(t, runningFilter.Insert(currBlockFilter, runningFilter.FromBlock()))

		iterator, err := cache.NewMatchedBlockIterator(runningFilter.FromBlock(), runningFilter.FromBlock(), maxScannedLimit, &eventMatcher, runningFilter)
		require.NoError(t, err)

		matchedBlock, ok, err := iterator.Next()
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, runningFilter.FromBlock(), matchedBlock)
	})

	t.Run("range with cache misses", func(t *testing.T) {
		// TODO(Ege)
	})
}
