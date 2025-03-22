package blockchain

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bloom/v3"
)

type EventMatcher struct {
	contractAddress *felt.Felt
	keysMap         []map[felt.Felt]struct{}
}

func NewEventMatcher(contractAddress *felt.Felt, keys [][]felt.Felt) EventMatcher {
	return EventMatcher{
		contractAddress: contractAddress,
		keysMap:         makeKeysMaps(keys),
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

func (e *EventMatcher) matchesEventKeys(eventKeys []*felt.Felt) bool {
	// short circuit if event doest have enough keys
	for i := len(eventKeys); i < len(e.keysMap); i++ {
		if len(e.keysMap[i]) > 0 {
			return false
		}
	}

	/// e.keys = [["V1", "V2"], [], ["V3"]] means:
	/// ((event.Keys[0] == "V1" OR event.Keys[0] == "V2") AND (event.Keys[2] == "V3")).
	//
	// Essentially
	// for each event.Keys[i], (len(e.keys[i]) == 0 OR event.Keys[i] is in e.keys[i]) should hold
	for index, eventKey := range eventKeys {
		// empty filter keys means match all
		if index >= len(e.keysMap) || len(e.keysMap[index]) == 0 {
			break
		}
		if _, found := e.keysMap[index][*eventKey]; !found {
			return false
		}
	}

	return true
}

func (e *EventMatcher) TestBloom(bloomFilter *bloom.BloomFilter) bool {
	possibleMatches := true
	if e.contractAddress != nil {
		addrBytes := e.contractAddress.Bytes()
		possibleMatches = bloomFilter.Test(addrBytes[:])
		// bloom filter says no events from this contract
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

func (e *EventMatcher) AppendBlockEvents(matchedEventsSofar []*FilteredEvent, header *core.Header, receipts []*core.TransactionReceipt,
	skippedEvents uint64, chunkSize uint64,
) ([]*FilteredEvent, uint64, error) {
	processedEvents := uint64(0)
	for _, receipt := range receipts {
		for i, event := range receipt.Events {
			var blockNumber *uint64
			// if header.Hash == nil it's a pending block
			if header.Hash != nil {
				blockNumber = &header.Number
			}

			// if last request was interrupted mid-block, and we are still processing that block, skip events
			// that were already processed
			if processedEvents < skippedEvents {
				processedEvents++
				continue
			}

			if e.contractAddress != nil && !event.From.Equal(e.contractAddress) {
				processedEvents++
				continue
			}

			if !e.matchesEventKeys(event.Keys) {
				processedEvents++
				continue
			}

			if uint64(len(matchedEventsSofar)) < chunkSize {
				matchedEventsSofar = append(matchedEventsSofar, &FilteredEvent{
					BlockNumber:     blockNumber,
					BlockHash:       header.Hash,
					TransactionHash: receipt.TransactionHash,
					EventIndex:      i,
					Event:           event,
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
