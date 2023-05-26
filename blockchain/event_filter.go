package blockchain

import (
	"errors"
	"fmt"

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
	keys            []*felt.Felt
}

type EventFilterRange uint

const (
	EventFilterFrom EventFilterRange = iota
	EventFilterTo
)

func newEventFilter(txn db.Transaction, contractAddress *felt.Felt, keys []*felt.Felt, fromBlock, toBlock uint64) *EventFilter {
	return &EventFilter{
		txn:             txn,
		contractAddress: contractAddress,
		keys:            keys,
		fromBlock:       fromBlock,
		toBlock:         toBlock,
	}
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

func (e *EventFilter) Events(cToken *ContinuationToken, chunkSize uint64) ([]*FilteredEvent, *ContinuationToken, error) {
	var matchedEvents []*FilteredEvent
	latest, err := chainHeight(e.txn)
	if err != nil {
		return nil, nil, err
	}

	var pending Pending
	if e.toBlock > latest {
		e.toBlock = latest + 1
		pending, err = pendingBlock(e.txn)
		if errors.Is(err, db.ErrKeyNotFound) {
			e.toBlock = latest
		} else if err != nil {
			return nil, nil, err
		}
	}

	filterKeysMap := make(map[felt.Felt]bool, len(e.keys))
	for _, key := range e.keys {
		filterKeysMap[*key] = true
	}

	curBlock := e.fromBlock
	// skip the blocks that we previously processed for this request
	if cToken != nil {
		curBlock = cToken.fromBlock
	}

	for ; curBlock <= e.toBlock; curBlock++ {
		var header *core.Header
		if curBlock != latest+1 {
			header, err = blockHeaderByNumber(e.txn, curBlock)
			if err != nil {
				return nil, nil, err
			}
		} else {
			header = pending.Block.Header
		}

		// bloom filter says no events match the filter, skip this block entirely if from is not nil
		if possibleMatches := e.testBloom(header.EventsBloom, filterKeysMap); !possibleMatches {
			continue
		}

		var receipts []*core.TransactionReceipt
		if curBlock != latest+1 {
			receipts, err = receiptsByBlockNumber(e.txn, header.Number)
			if err != nil {
				return nil, nil, err
			}
		} else {
			receipts = pending.Block.Receipts
		}

		var processedEvents uint64
		matchedEvents, processedEvents, err = e.appendBlockEvents(matchedEvents, header, receipts, filterKeysMap, cToken, chunkSize)
		if err != nil {
			if errors.Is(err, errChunkSizeReached) {
				return matchedEvents, &ContinuationToken{
					fromBlock:       curBlock,
					processedEvents: processedEvents,
				}, nil
			}
			return nil, nil, err
		}
	}
	return matchedEvents, nil, nil
}

func (e *EventFilter) testBloom(bloomFilter *bloom.BloomFilter, keysMap map[felt.Felt]bool) bool {
	possibleMatches := true
	if e.contractAddress != nil {
		addrBytes := e.contractAddress.Bytes()
		possibleMatches = bloomFilter.Test(addrBytes[:])
		// bloom filter says no events from this contract
		if !possibleMatches {
			return possibleMatches
		}
	}
	for key := range keysMap {
		keyBytes := key.Bytes()

		// check if block possibly contains the event we are looking for
		possibleMatches = bloomFilter.Test(keyBytes[:])
		if possibleMatches {
			return possibleMatches
		}
	}

	return possibleMatches
}

func (e *EventFilter) appendBlockEvents(matchedEventsSofar []*FilteredEvent, header *core.Header,
	receipts []*core.TransactionReceipt, keysMap map[felt.Felt]bool, cToken *ContinuationToken, chunkSize uint64,
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

			matches := len(e.keys) == 0 // empty filter keys means match all
			for _, eventKey := range event.Keys {
				if matches {
					break
				}
				_, matches = keysMap[*eventKey]
			}

			if matches {
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
