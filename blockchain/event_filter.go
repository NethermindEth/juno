package blockchain

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

var errChunkSizeReached = errors.New("chunk size reached")

//go:generate mockgen -destination=../mocks/mock_event_filterer.go -package=mocks github.com/NethermindEth/juno/blockchain EventFilterer
type EventFilterer interface {
	io.Closer

	Events(cToken *ContinuationToken, chunkSize uint64) ([]*FilteredEvent, *ContinuationToken, error)
	SetRangeEndBlockByNumber(filterRange EventFilterRange, blockNumber uint64) error
	SetRangeEndBlockByHash(filterRange EventFilterRange, blockHash *felt.Felt) error
	WithLimit(limit uint) *EventFilter
}

type EventFilter struct {
	txn            db.Transaction
	fromBlock      uint64
	toBlock        uint64
	matcher        EventMatcher
	maxScanned     uint // maximum number of scanned blocks in single call.
	pendingBlockFn func() *core.Block
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
		txn:            txn,
		matcher:        NewEventMatcher(contractAddress, keys),
		fromBlock:      fromBlock,
		toBlock:        toBlock,
		maxScanned:     math.MaxUint,
		pendingBlockFn: pendingBlockFn,
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
	BlockNumber     *uint64
	BlockHash       *felt.Felt
	TransactionHash *felt.Felt
	EventIndex      int
}

//nolint:gocyclo
func (e *EventFilter) Events(cToken *ContinuationToken, chunkSize uint64) ([]*FilteredEvent, *ContinuationToken, error) {
	var matchedEvents []*FilteredEvent
	latest, err := core.ChainHeight(e.txn)
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

	var skippedEvents uint64
	curBlock := e.fromBlock
	// skip the blocks that we previously processed for this request
	if cToken != nil {
		skippedEvents = cToken.processedEvents
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

		if possibleMatches := e.matcher.TestBloom(header.EventsBloom); !possibleMatches {
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
		matchedEvents, processedEvents, err = e.matcher.AppendBlockEvents(matchedEvents, header, receipts, skippedEvents, chunkSize)
		if err != nil {
			if errors.Is(err, errChunkSizeReached) {
				rToken = &ContinuationToken{fromBlock: curBlock, processedEvents: processedEvents}
				break
			}
			return nil, nil, err
		}

		// Skipped events are processed, so we can reset the counter
		skippedEvents = 0
	}

	if rToken == nil && remainingScannedBlocks == 0 && curBlock <= e.toBlock {
		rToken = &ContinuationToken{fromBlock: curBlock, processedEvents: 0}
	}
	return matchedEvents, rToken, nil
}
