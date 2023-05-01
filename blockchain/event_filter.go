package blockchain

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

var errChunkSizeReached = errors.New("chunk size reached")

type EventFilter struct {
	txn       db.Transaction
	FromBlock uint64
	ToBlock   uint64
	From      *felt.Felt
	Keys      []*felt.Felt
}

type FilterRangeEnd byte

const (
	From FilterRangeEnd = iota
	To
)

func NewEventFilter(txn db.Transaction, from *felt.Felt, keys []*felt.Felt) *EventFilter {
	return &EventFilter{
		txn:  txn,
		From: from,
		Keys: keys,
	}
}

// SetRangeEndBlockByNumber sets an end of the block range by block number
func (f *EventFilter) SetRangeEndBlockByNumber(end FilterRangeEnd, blockNumber uint64) error {
	_, err := blockHeaderByNumber(f.txn, blockNumber)
	if err != nil {
		return err
	}
	if end == From {
		f.FromBlock = blockNumber
	} else if end == To {
		f.ToBlock = blockNumber
	} else {
		return errors.New("undefined range end")
	}
	return nil
}

// SetRangeEndBlockByHash sets an end of the block range by block hash
func (f *EventFilter) SetRangeEndBlockByHash(end FilterRangeEnd, blockHash *felt.Felt) error {
	header, err := blockHeaderByHash(f.txn, blockHash)
	if err != nil {
		return err
	}
	return f.SetRangeEndBlockByNumber(end, header.Number)
}

// Close closes the underlying database transaction that provides the blockchain snapshot
func (f *EventFilter) Close() error {
	return f.txn.Discard()
}

type ContinuationToken struct {
	fromBlock       uint64
	processedEvents uint64
}

func (t *ContinuationToken) String() string {
	return fmt.Sprintf("%d-%d", t.fromBlock, t.processedEvents)
}

func (t *ContinuationToken) FromString(str string) error {
	_, err := fmt.Sscanf(str, "%d-%d", &t.fromBlock, &t.processedEvents)
	return err
}

type FilteredEvent struct {
	*core.Event
	BlockNumber     uint64
	BlockHash       *felt.Felt
	TransactionHash *felt.Felt
}

func (f *EventFilter) Events(cToken *ContinuationToken, chunkSize uint64) ([]*FilteredEvent, *ContinuationToken, error) {
	var matchedEvents []*FilteredEvent

	var bloomLogBuffer [felt.Bytes * 2]byte
	fromBytes := f.From.Bytes()
	copy(bloomLogBuffer[:felt.Bytes], fromBytes[:])

	filterKeysMap := make(map[felt.Felt]bool, len(f.Keys))
	for _, key := range f.Keys {
		filterKeysMap[*key] = true
	}

	for curBlock := f.FromBlock; curBlock <= f.ToBlock; curBlock++ {
		// skip the blocks that we previously processed for this request
		if cToken != nil && curBlock < cToken.fromBlock {
			continue
		}

		header, err := blockHeaderByNumber(f.txn, curBlock)
		if err != nil {
			return nil, nil, err
		}

		// test `from` only by default because empty filter keys means match all
		possiblyMatches := header.EventsBloom.Test(fromBytes[:])
		for key := range filterKeysMap {
			keyBytes := key.Bytes()
			copy(bloomLogBuffer[felt.Bytes:], keyBytes[:])

			// check if block possibly contains the event we are looking for
			possiblyMatches = header.EventsBloom.Test(bloomLogBuffer[:])
			if possiblyMatches {
				break
			}
		}

		// bloom filter says no events match the filter, skip this block entirely
		if !possiblyMatches {
			continue
		}

		var processedEvents uint64
		matchedEvents, processedEvents, err = f.appendBlockEvents(matchedEvents, header, filterKeysMap, cToken, chunkSize)
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

func (f *EventFilter) appendBlockEvents(matchedEventsSofar []*FilteredEvent, header *core.Header,
	keysMap map[felt.Felt]bool, cToken *ContinuationToken, chunkSize uint64,
) ([]*FilteredEvent, uint64, error) {
	receipts, err := receiptsByBlockNumber(f.txn, header.Number)
	if err != nil {
		return nil, 0, err
	}

	processedEvents := uint64(0)
	for _, receipt := range receipts {
		for _, event := range receipt.Events {
			// if last request was interrupted mid-block, and we are still processing that block, skip events
			// that were already processed
			if cToken != nil && header.Number == cToken.fromBlock && processedEvents < cToken.processedEvents {
				processedEvents++
				continue
			}

			if !event.From.Equal(f.From) {
				processedEvents++
				continue
			}

			matches := len(f.Keys) == 0 // empty filter keys means match all
			for _, eventKeys := range event.Keys {
				if matches {
					break
				}
				_, matches = keysMap[*eventKeys]
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
