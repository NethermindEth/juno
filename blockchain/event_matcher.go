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

// matchesAllBlocks reports whether every block is a candidate at the bloom
// level: no addresses and no keys at any position. Such a matcher never needs
// bloom data.
func (e *EventMatcher) matchesAllBlocks() bool {
	if len(e.contractAddressBytes) > 0 {
		return false
	}
	for _, kMap := range e.keysMap {
		if len(kMap) > 0 {
			return false
		}
	}
	return true
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

// AppendBlockEventsFromTransactionEvents appends the events of one block that match
// the filter to matchedEventsSofar. blockEvents holds the events-only projection of
// the block's receipts. blockHashFn gets the block hash on the first match, so a
// block with no match reads no header.
//
// AppendBlockEventsFromReceipts holds the same loop for full receipts. Change both
// together: a shared loop needs a per-transaction call the compiler cannot inline.
func (e *EventMatcher) AppendBlockEventsFromTransactionEvents(
	matchedEventsSofar []FilteredEvent,
	blockNum uint64,
	blockHashFn func() (*felt.Felt, error),
	blockEvents []core.TransactionEvents,
	skippedEvents uint64,
	chunkSize uint64,
) ([]FilteredEvent, uint64, error) {
	var (
		blockHash    *felt.Felt
		hashResolved bool
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
				if !hashResolved {
					var err error
					if blockHash, err = blockHashFn(); err != nil {
						return nil, 0, err
					}
					hashResolved = true
				}
				matchedEventsSofar = append(matchedEventsSofar, FilteredEvent{
					BlockNumber:      blockNum,
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

// AppendBlockEventsFromReceipts appends the events of one block that match the filter
// to matchedEventsSofar, read from full receipts of any source. The caller holds the
// block hash, which is nil for a source that has none, so this loop takes it directly.
// See AppendBlockEventsFromTransactionEvents for why the loops stay separate.
func (e *EventMatcher) AppendBlockEventsFromReceipts(
	matchedEventsSofar []FilteredEvent,
	blockNum uint64,
	blockHash *felt.Felt,
	receipts []*core.TransactionReceipt,
	skippedEvents uint64,
	chunkSize uint64,
) ([]FilteredEvent, uint64, error) {
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
				matchedEventsSofar = append(matchedEventsSofar, FilteredEvent{
					BlockNumber:      blockNum,
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
