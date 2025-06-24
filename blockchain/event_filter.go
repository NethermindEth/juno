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
	txn            db.KeyValueStore
	fromBlock      uint64
	toBlock        uint64
	matcher        EventMatcher
	maxScanned     uint // maximum number of scanned blocks in single call.
	pendingBlockFn func() *core.Block
	cachedFilters  *AggregatedBloomFilterCache
	runningFilter  *core.RunningEventFilter
}

type EventFilterRange uint

const (
	EventFilterFrom EventFilterRange = iota
	EventFilterTo
)

func newEventFilter(
	txn db.KeyValueStore,
	contractAddress *felt.Felt,
	keys [][]felt.Felt,
	fromBlock, toBlock uint64,
	pendingBlockFn func() *core.Block,
	cachedFilters *AggregatedBloomFilterCache,
	runningFilter *core.RunningEventFilter,
) *EventFilter {
	return &EventFilter{
		txn:            txn,
		matcher:        NewEventMatcher(contractAddress, keys),
		fromBlock:      fromBlock,
		toBlock:        toBlock,
		maxScanned:     math.MaxUint,
		pendingBlockFn: pendingBlockFn,
		cachedFilters:  cachedFilters,
		runningFilter:  runningFilter,
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
	header, err := core.GetBlockHeaderByHash(e.txn, blockHash)
	if err != nil {
		return err
	}
	return e.SetRangeEndBlockByNumber(filterRange, header.Number)
}

// Close closes the underlying database transaction that provides the blockchain snapshot
func (e *EventFilter) Close() error {
	return nil // no-op
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

func (e *EventFilter) Events(cToken *ContinuationToken, chunkSize uint64) ([]*FilteredEvent, *ContinuationToken, error) {
	var matchedEvents []*FilteredEvent

	latest, err := core.GetChainHeight(e.txn)
	if err != nil {
		return nil, nil, err
	}

	e.toBlock = min(e.toBlock, latest+1)

	var skippedEvents uint64
	curBlock := e.fromBlock
	// skip the blocks that we previously processed for this request
	if cToken != nil {
		skippedEvents = cToken.processedEvents
		curBlock = cToken.fromBlock
	}

	// only canonical blocks
	if e.toBlock <= latest {
		return e.canonicalEvents(matchedEvents, curBlock, e.toBlock, skippedEvents, chunkSize)
	}

	var rToken *ContinuationToken
	// [canonicalBlock, pending]
	if curBlock <= latest {
		matchedEvents, rToken, err = e.canonicalEvents(matchedEvents, curBlock, latest, skippedEvents, chunkSize)
		if err != nil {
			return nil, nil, err
		}

		if rToken != nil {
			return matchedEvents, rToken, nil
		}
		// Skipped events are processed, so we can reset the counter
		skippedEvents = 0
	}

	// Case [canonicalBlock, pending] || [pending, pending]
	pending := e.pendingBlockFn()
	if pending == nil {
		return matchedEvents, nil, nil
	}

	header := pending.Header
	if possibleMatches := e.matcher.TestBloom(header.EventsBloom); !possibleMatches {
		return matchedEvents, nil, nil
	}

	var processedEvents uint64
	matchedEvents, processedEvents, err = e.matcher.AppendBlockEvents(matchedEvents, header, pending.Receipts, skippedEvents, chunkSize)
	if err != nil {
		// Max events to scan exhausted middle of pending block, continue from next unprocessed event
		if errors.Is(err, errChunkSizeReached) {
			rToken := &ContinuationToken{fromBlock: latest + 1, processedEvents: processedEvents}
			return matchedEvents, rToken, nil
		}
		return nil, nil, err
	}

	return matchedEvents, nil, nil
}

func (e *EventFilter) canonicalEvents(
	matchedEvents []*FilteredEvent,
	fromBlock, toBlock, skippedEvents, chunkSize uint64,
) ([]*FilteredEvent, *ContinuationToken, error) {
	matchedBlockIter, err := e.cachedFilters.NewMatchedBlockIterator(
		fromBlock,
		toBlock,
		uint64(e.maxScanned),
		&e.matcher,
		e.runningFilter,
	)
	if err != nil {
		return nil, nil, err
	}

	var (
		rToken              *ContinuationToken
		lastProccessedBlock uint64
	)
	for {
		curBlock, ok, err := matchedBlockIter.Next()
		if !ok {
			if err == nil {
				break
			}
			// If max scans exhausted end of block
			if errors.Is(err, ErrMaxScannedBlockLimitExceed) {
				// Next candidate block for continuation token
				lastProccessedBlock = curBlock
				break
			}
			return nil, nil, err
		}

		lastProccessedBlock = curBlock

		var header *core.Header
		header, err = core.GetBlockHeaderByNumber(e.txn, curBlock)
		if err != nil {
			return nil, nil, err
		}

		var receipts []*core.TransactionReceipt
		receipts, err = core.GetReceiptsByBlockNum(e.txn, header.Number)
		if err != nil {
			return nil, nil, err
		}

		var processedEvents uint64
		matchedEvents, processedEvents, err = e.matcher.AppendBlockEvents(matchedEvents, header, receipts, skippedEvents, chunkSize)
		if err != nil {
			// Max events to scan exhausted mid block, continue from next unprocessed event
			if errors.Is(err, errChunkSizeReached) {
				rToken = &ContinuationToken{fromBlock: curBlock, processedEvents: processedEvents}
				return matchedEvents, rToken, nil
			}
			return nil, nil, err
		}

		// Skipped events are processed, so we can reset the counter
		skippedEvents = 0
	}

	// If max scans exhausted end of block
	if matchedBlockIter.scannedCount > matchedBlockIter.maxScanned && lastProccessedBlock <= e.toBlock {
		rToken = &ContinuationToken{fromBlock: lastProccessedBlock, processedEvents: 0}
		return matchedEvents, rToken, nil
	}

	return matchedEvents, nil, nil
}
