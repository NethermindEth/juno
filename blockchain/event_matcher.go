package blockchain

import (
	"encoding/binary"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
)

type EventMatcher struct {
	contractAddresses    []felt.Address
	contractAddressBytes [][]byte
	keysMap              []map[felt.Felt]struct{}
}

func NewEventMatcher(contractAddresses []felt.Address, keys [][]felt.Felt) EventMatcher {
	contractAddressBytes := make([][]byte, len(contractAddresses))
	for i, addr := range contractAddresses {
		b := addr.Bytes()
		contractAddressBytes[i] = append([]byte(nil), b[:]...)
	}
	return EventMatcher{
		contractAddresses:    contractAddresses,
		contractAddressBytes: contractAddressBytes,
		keysMap:              makeKeysMaps(keys),
	}
}

func makeKeysMaps(filterKeys [][]felt.Felt) []map[felt.Felt]struct{} {
	filterKeysMaps := make([]map[felt.Felt]struct{}, len(filterKeys))
	for index, keys := range filterKeys {
		kMap := make(map[felt.Felt]struct{}, len(keys))
		for _, key := range keys {
			kMap[key] = struct{}{}
		}
		filterKeysMaps[index] = kMap
	}

	return filterKeysMaps
}

func (e *EventMatcher) MatchesEventKeys(eventKeys []felt.Felt) bool {
	// short circuit if event doest have enough keys
	if len(eventKeys) < len(e.keysMap) {
		return false
	}

	/// e.keys = [["V1", "V2"], [], ["V3"]] means:
	/// ((event.Keys[0] == "V1" OR event.Keys[0] == "V2") AND (event.Keys[2] == "V3")).
	//
	// Essentially
	// for each event.Keys[i], (len(e.keys[i]) == 0 OR event.Keys[i] is in e.keys[i]) should hold
	for index, eventKey := range eventKeys {
		if index >= len(e.keysMap) {
			// event has more keys than filter keys and
			// so far event keys match the filter keys
			return true
		}
		// empty filter keys means match all
		if len(e.keysMap[index]) == 0 {
			continue
		}
		// check if event key is in filter keys
		if _, found := e.keysMap[index][eventKey]; !found {
			return false
		}
	}

	return true
}

func (e *EventMatcher) TestBloom(bloomFilter *bloom.BloomFilter) bool {
	possibleMatches := true
	if len(e.contractAddressBytes) > 0 {
		possibleMatches = slices.ContainsFunc(e.contractAddressBytes, bloomFilter.Test)
		// bloom filter says no events from any of these contracts
		if !possibleMatches {
			return possibleMatches
		}
	}

	for index, kMap := range e.keysMap {
		for key := range kMap {
			keyBytes := key.Bytes()
			keyAndIndexBytes := binary.AppendVarint(keyBytes[:], int64(index))

			// check if block possibly contains the event we are looking for
			possibleMatches = bloomFilter.Test(keyAndIndexBytes)
			// possible match for this index, no need to continue checking the rest of the keys
			if possibleMatches {
				break
			}
		}

		// no key on this index matches the filter
		if !possibleMatches {
			break
		}
	}

	return possibleMatches
}

// Returns candidate possibly matching block in the given filter.
func (e *EventMatcher) getCandidateBlocksForFilterInto(filter *core.AggregatedBloomFilter, out *bitset.BitSet) error {
	if out == nil {
		return core.ErrMatchesBufferNil
	}

	if out.Len() != uint(core.NumBlocksPerFilter) {
		return core.ErrMatchesBufferSizeMismatch
	}

	out.SetAll()

	innerMatch := bitset.New(uint(core.NumBlocksPerFilter))
	if len(e.contractAddressBytes) > 0 {
		if err := filter.BlocksForKeysInto(e.contractAddressBytes, innerMatch); err != nil {
			return err
		}

		out.InPlaceIntersection(innerMatch)

		if out.None() {
			return nil
		}
	}

	for index, kMap := range e.keysMap {
		keys := make([][]byte, 0, len(kMap))
		for key := range kMap {
			keyBytes := key.Bytes()
			keyAndIndex := binary.AppendVarint(keyBytes[:], int64(index))
			keys = append(keys, keyAndIndex)
		}

		if err := filter.BlocksForKeysInto(keys, innerMatch); err != nil {
			return err
		}

		out.InPlaceIntersection(innerMatch)
		if out.None() {
			return nil
		}
	}

	return nil
}

// AppendBlockEventsFromTransactionEvents appends the events of a canonical block that
// match the filter to matchedEventsSofar. Canonical receipts decode to the events subset
// only. blockHashFn gets the block hash. The code calls it on the first match only, so a
// block with no match reads no header.
//
// AppendBlockEventsFromReceipts holds the same loop for pre-confirmed blocks. Change both
// functions together. One shared loop needs a call for each transaction, and the compiler
// cannot inline that call. This costs 8-15% on both paths.
// TestEventMatcher_BothSourcesAgree compares the two functions.
//
//nolint:dupl // The two sources keep separate loops on purpose. See above.
func (e *EventMatcher) AppendBlockEventsFromTransactionEvents(
	matchedEventsSofar []FilteredEvent,
	blockNum uint64,
	blockHashFn func() (*felt.Felt, error),
	blockEvents []core.TransactionEvents,
	skippedEvents uint64,
	chunkSize uint64,
) ([]FilteredEvent, uint64, error) {
	var (
		blockHash   *felt.Felt
		blockNumPtr *uint64
	)
	processedEvents := uint64(0)
	for txIndex, txEvents := range blockEvents {
		txHash, events := txEvents.TransactionHash, txEvents.Events
		for i, event := range events {
			// if last request was interrupted mid-block, and we are still processing that block, skip events
			// that were already processed
			if processedEvents < skippedEvents {
				processedEvents++
				continue
			}

			if len(e.contractAddresses) > 0 {
				// todo: remove the cast to felt.Address
				contains := slices.Contains(e.contractAddresses, felt.Address(*event.From))
				if !contains {
					processedEvents++
					continue
				}
			}

			if !e.MatchesEventKeys(event.Keys) {
				processedEvents++
				continue
			}

			if uint64(len(matchedEventsSofar)) < chunkSize {
				if blockNumPtr == nil {
					var err error
					if blockHash, err = blockHashFn(); err != nil {
						return nil, 0, err
					}
					// Copy the block number. The address of a parameter moves the
					// parameter to the heap at function entry, also for a block
					// with no match.
					num := blockNum
					blockNumPtr = &num
				}
				matchedEventsSofar = append(matchedEventsSofar, FilteredEvent{
					BlockNumber:      blockNumPtr,
					BlockHash:        blockHash,
					TransactionHash:  txHash,
					TransactionIndex: uint(txIndex),
					EventIndex:       uint(i),
					Event:            event,
				})
			} else {
				// we are at the capacity, return what we have accumulated so far and a continuation token
				return matchedEventsSofar, processedEvents, errChunkSizeReached
			}
			// count the events we processed for this block to include in the continuation token
			processedEvents++
		}
	}
	return matchedEventsSofar, processedEvents, nil
}

// AppendBlockEventsFromReceipts appends the events of a pre-confirmed block that match
// the filter to matchedEventsSofar. A pre-confirmed block holds full receipts in memory.
// This function reads the events from the receipts, so the path needs no projection.
// See AppendBlockEventsFromTransactionEvents for the reason the loops stay separate.
//
//nolint:dupl // The other source keeps its own loop on purpose. See above.
func (e *EventMatcher) AppendBlockEventsFromReceipts(
	matchedEventsSofar []FilteredEvent,
	blockNum uint64,
	blockHashFn func() (*felt.Felt, error),
	receipts []*core.TransactionReceipt,
	skippedEvents uint64,
	chunkSize uint64,
) ([]FilteredEvent, uint64, error) {
	var (
		blockHash   *felt.Felt
		blockNumPtr *uint64
	)
	processedEvents := uint64(0)
	for txIndex, receipt := range receipts {
		txHash, events := receipt.TransactionHash, receipt.Events
		for i, event := range events {
			if processedEvents < skippedEvents {
				processedEvents++
				continue
			}

			if len(e.contractAddresses) > 0 {
				contains := slices.Contains(e.contractAddresses, felt.Address(*event.From))
				if !contains {
					processedEvents++
					continue
				}
			}

			if !e.MatchesEventKeys(event.Keys) {
				processedEvents++
				continue
			}

			if uint64(len(matchedEventsSofar)) < chunkSize {
				if blockNumPtr == nil {
					var err error
					if blockHash, err = blockHashFn(); err != nil {
						return nil, 0, err
					}
					num := blockNum
					blockNumPtr = &num
				}
				matchedEventsSofar = append(matchedEventsSofar, FilteredEvent{
					BlockNumber:      blockNumPtr,
					BlockHash:        blockHash,
					TransactionHash:  txHash,
					TransactionIndex: uint(txIndex),
					EventIndex:       uint(i),
					Event:            event,
				})
			} else {
				// we are at the capacity, return what we have accumulated so far and a continuation token
				return matchedEventsSofar, processedEvents, errChunkSizeReached
			}
			// count the events we processed for this block to include in the continuation token
			processedEvents++
		}
	}
	return matchedEventsSofar, processedEvents, nil
}

func (e *EventMatcher) MatchesAddress(eventFrom *felt.Felt) bool {
	if len(e.contractAddresses) == 0 {
		return true
	}
	if eventFrom == nil {
		return false
	}
	return slices.Contains(e.contractAddresses, felt.Address(*eventFrom))
}
