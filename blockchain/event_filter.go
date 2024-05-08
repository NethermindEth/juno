package blockchain

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bloom/v3"
)

var errChunkSizeReached = errors.New("chunk size reached")

type EventFilter struct {
	txn             db.Transaction
	fromBlock       uint64
	toBlock         uint64
	contractAddress *felt.Felt
	keys            [][]felt.Felt
	maxScanned      uint // maximum number of scanned blocks in single call.
	pendingBlockFn  func() *core.Block
}

type EventFilterRange uint

const (
	EventFilterFrom EventFilterRange = iota
	EventFilterTo
)

func newEventFilter(txn db.Transaction, contractAddress *felt.Felt, keys [][]felt.Felt, fromBlock, toBlock uint64,
	pendingBlockFn func() *core.Block,
) *EventFilter {
	return &EventFilter{
		txn:             txn,
		contractAddress: contractAddress,
		keys:            keys,
		fromBlock:       fromBlock,
		toBlock:         toBlock,
		maxScanned:      math.MaxUint,
		pendingBlockFn:  pendingBlockFn,
	}
}

// WithLimit sets the limit for events scan
func (e *EventFilter) WithLimit(limit uint) *EventFilter {
	e.maxScanned = limit
	return e
}

// SetRangeEndBlockByNumber sets an end of the block range by block number
func (e *EventFilter) SetRangeEndBlockByNumber(filterRange EventFilterRange, blockNumber uint64) error {
	if filterRange == EventFilterFrom {
		e.fromBlock = blockNumber
	} else if filterRange == EventFilterTo {
		e.toBlock = blockNumber
	} else {
		return errors.New("undefined range end")
	}
	return nil
}

// SetRangeEndBlockByHash sets an end of the block range by block hash
func (e *EventFilter) SetRangeEndBlockByHash(filterRange EventFilterRange, blockHash *felt.Felt) error {
	header, err := blockHeaderByHash(e.txn, blockHash)
	if err != nil {
		return err
	}
	return e.SetRangeEndBlockByNumber(filterRange, header.Number)
}

// Close closes the underlying database transaction that provides the blockchain snapshot
func (e *EventFilter) Close() error {
	return e.txn.Discard()
}

type ContinuationToken struct {
	fromBlock       uint64
	processedEvents uint64
}

func (c *ContinuationToken) String() string {
	return fmt.Sprintf("%d-%d", c.fromBlock, c.processedEvents)
}

func (c *ContinuationToken) FromString(str string) error {
	_, err := fmt.Sscanf(str, "%d-%d", &c.fromBlock, &c.processedEvents)
	return err
}

type FilteredEvent struct {
	*core.Event
	BlockNumber     uint64
	BlockHash       *felt.Felt
	TransactionHash *felt.Felt
}

//nolint:gocyclo
func (e *EventFilter) Events(cToken *ContinuationToken, chunkSize uint64) ([]*FilteredEvent, *ContinuationToken, error) {
	var matchedEvents []*FilteredEvent
	latest, err := chainHeight(e.txn)
	if err != nil {
		return nil, nil, err
	}

	var pending *core.Block
	if e.toBlock > latest {
		e.toBlock = latest + 1

		pending = e.pendingBlockFn()
		if pending == nil {
			e.toBlock = latest
		}
	}

	filterKeysMaps := makeKeysMaps(e.keys)

	curBlock := e.fromBlock
	// skip the blocks that we previously processed for this request
	if cToken != nil {
		curBlock = cToken.fromBlock
	}

	var (
		remainingScannedBlocks = e.maxScanned
		rToken                 *ContinuationToken
	)
	for ; curBlock <= e.toBlock && remainingScannedBlocks > 0; curBlock, remainingScannedBlocks = curBlock+1, remainingScannedBlocks-1 {
		var header *core.Header
		if curBlock != latest+1 {
			header, err = blockHeaderByNumber(e.txn, curBlock)
			if err != nil {
				return nil, nil, err
			}
		} else {
			header = pending.Header
		}

		if possibleMatches := e.testBloom(header.EventsBloom, filterKeysMaps); !possibleMatches {
			// bloom filter says no events match the filter, skip this block entirely if from is not nil
			continue
		}

		var receipts []*core.TransactionReceipt
		if curBlock != latest+1 {
			receipts, err = receiptsByBlockNumber(e.txn, header.Number)
			if err != nil {
				return nil, nil, err
			}
		} else {
			receipts = pending.Receipts
		}

		var processedEvents uint64
		matchedEvents, processedEvents, err = e.appendBlockEvents(matchedEvents, header, receipts, filterKeysMaps, cToken, chunkSize)
		if err != nil {
			if errors.Is(err, errChunkSizeReached) {
				rToken = &ContinuationToken{fromBlock: curBlock, processedEvents: processedEvents}
				break
			}
			return nil, nil, err
		}
	}

	if rToken == nil && remainingScannedBlocks == 0 && curBlock <= e.toBlock {
		rToken = &ContinuationToken{fromBlock: curBlock, processedEvents: 0}
	}
	return matchedEvents, rToken, nil
}

func (e *EventFilter) testBloom(bloomFilter *bloom.BloomFilter, keysMap []map[felt.Felt]struct{}) bool {
	possibleMatches := true
	if e.contractAddress != nil {
		addrBytes := e.contractAddress.Bytes()
		possibleMatches = bloomFilter.Test(addrBytes[:])
		// bloom filter says no events from this contract
		if !possibleMatches {
			return possibleMatches
		}
	}

	for index, kMap := range keysMap {
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

func (e *EventFilter) appendBlockEvents(matchedEventsSofar []*FilteredEvent, header *core.Header,
	receipts []*core.TransactionReceipt, keysMap []map[felt.Felt]struct{}, cToken *ContinuationToken, chunkSize uint64,
) ([]*FilteredEvent, uint64, error) {
	processedEvents := uint64(0)
	for _, receipt := range receipts {
		for _, event := range receipt.Events {
			// if last request was interrupted mid-block, and we are still processing that block, skip events
			// that were already processed
			if cToken != nil && header.Number == cToken.fromBlock && processedEvents < cToken.processedEvents {
				processedEvents++
				continue
			}

			if e.contractAddress != nil && !event.From.Equal(e.contractAddress) {
				processedEvents++
				continue
			}

			if e.matchesEventKeys(event.Keys, keysMap) {
				if uint64(len(matchedEventsSofar)) < chunkSize {
					matchedEventsSofar = append(matchedEventsSofar, &FilteredEvent{
						BlockNumber:     header.Number,
						BlockHash:       header.Hash,
						TransactionHash: receipt.TransactionHash,
						Event:           event,
					})
				} else {
					// we are at the capacity, return what we have accumulated so far and a continuation token
					return matchedEventsSofar, processedEvents, errChunkSizeReached
				}
			}
			// count the events we processed for this block to include in the continuation token
			processedEvents++
		}
	}
	return matchedEventsSofar, processedEvents, nil
}

func (e *EventFilter) matchesEventKeys(eventKeys []*felt.Felt, keysMap []map[felt.Felt]struct{}) bool {
	// short circuit if event doest have enough keys
	for i := len(eventKeys); i < len(keysMap); i++ {
		if len(keysMap[i]) > 0 {
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
		if index >= len(keysMap) || len(keysMap[index]) == 0 {
			break
		}
		if _, found := keysMap[index][*eventKey]; !found {
			return false
		}
	}

	return true
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
