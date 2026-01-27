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

	Events(cToken *ContinuationToken, chunkSize uint64) ([]FilteredEvent, ContinuationToken, error)
	SetRangeEndBlockByNumber(filterRange EventFilterRange, blockNumber uint64) error
	SetRangeEndBlockByHash(filterRange EventFilterRange, blockHash *felt.Felt) error
	SetRangeEndBlockToL1Head(filterRange EventFilterRange) error
	WithLimit(limit uint) *EventFilter
}

type EventFilter struct {
	txn           db.KeyValueStore
	fromBlock     uint64
	toBlock       uint64
	matcher       EventMatcher
	maxScanned    uint // maximum number of scanned blocks in single call.
	pendingDataFn func() (core.PendingData, error)
	cachedFilters *AggregatedBloomFilterCache
	runningFilter *core.RunningEventFilter
}

type EventFilterRange uint

const (
	EventFilterFrom EventFilterRange = iota
	EventFilterTo
)

func newEventFilter(
	txn db.KeyValueStore,
	contractAddresses []felt.Felt,
	keys [][]felt.Felt,
	fromBlock, toBlock uint64,
	pendingDataFn func() (core.PendingData, error),
	cachedFilters *AggregatedBloomFilterCache,
	runningFilter *core.RunningEventFilter,
) *EventFilter {
	return &EventFilter{
		txn:           txn,
		matcher:       NewEventMatcher(contractAddresses, keys),
		fromBlock:     fromBlock,
		toBlock:       toBlock,
		maxScanned:    math.MaxUint,
		pendingDataFn: pendingDataFn,
		cachedFilters: cachedFilters,
		runningFilter: runningFilter,
	}
}

// WithLimit sets the limit for events scan
func (e *EventFilter) WithLimit(limit uint) *EventFilter {
	e.maxScanned = limit
	return e
}

// SetRangeEndBlockByNumber sets an end of the block range by block number
func (e *EventFilter) SetRangeEndBlockByNumber(
	filterRange EventFilterRange,
	blockNumber uint64,
) error {
	switch filterRange {
	case EventFilterFrom:
		e.fromBlock = blockNumber
	case EventFilterTo:
		e.toBlock = blockNumber
	default:
		return errors.New("undefined range end")
	}
	return nil
}

// SetRangeEndBlockByHash sets an end of the block range by block hash
func (e *EventFilter) SetRangeEndBlockByHash(
	filterRange EventFilterRange,
	blockHash *felt.Felt,
) error {
	header, err := core.GetBlockHeaderByHash(e.txn, blockHash)
	if err != nil {
		return err
	}
	return e.SetRangeEndBlockByNumber(filterRange, header.Number)
}

// SetRangeEndBlockToL1Head sets an end of the block range to latest `l1_accepted` block
func (e *EventFilter) SetRangeEndBlockToL1Head(filterRange EventFilterRange) error {
	l1Head, err := core.GetL1Head(e.txn)
	if err != nil {
		return err
	}
	return e.SetRangeEndBlockByNumber(filterRange, l1Head.BlockNumber)
}

// Close closes the underlying database transaction that provides the blockchain snapshot
func (e *EventFilter) Close() error {
	return nil // no-op
}

type ContinuationToken struct {
	fromBlock       uint64
	processedEvents uint64
}

func (c *ContinuationToken) IsEmpty() bool {
	return c.fromBlock == 0 && c.processedEvents == 0
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
	BlockNumber *uint64
	BlockHash   *felt.Felt
	// BlockParentHash is used to distinguish pre_latest from pre_confirmed blocks
	// when assigning finality status.
	// If BlockNumber > latest_canonical_block_number or block hash is nil:
	//   - BlockParentHash == nil indicates pre_confirmed block
	//   - BlockParentHash != nil indicates pre_latest block
	BlockParentHash  *felt.Felt
	TransactionHash  *felt.Felt
	TransactionIndex uint
	EventIndex       uint
}

func (e *EventFilter) Events(
	cToken *ContinuationToken,
	chunkSize uint64,
) ([]FilteredEvent, ContinuationToken, error) {
	var matchedEvents []FilteredEvent

	latest, err := core.GetChainHeight(e.txn)
	if err != nil {
		return nil, ContinuationToken{}, err
	}

	var skippedEvents uint64
	startBlock := e.fromBlock
	// skip the blocks that we previously processed for this request
	if cToken != nil {
		skippedEvents = cToken.processedEvents
		startBlock = cToken.fromBlock
	}

	// Case [canonicalBlock, canonicalBlock]
	if e.toBlock <= latest {
		return e.canonicalEvents(
			matchedEvents,
			startBlock,
			e.toBlock,
			skippedEvents,
			chunkSize,
		)
	}

	// Case [canonicalBlock, pre-confirmed]
	if startBlock <= latest {
		var cToken ContinuationToken
		matchedEvents, cToken, err = e.canonicalEvents(
			matchedEvents,
			startBlock,
			latest,
			skippedEvents,
			chunkSize,
		)
		if err != nil {
			return nil, ContinuationToken{}, err
		}

		if !cToken.IsEmpty() {
			return matchedEvents, cToken, nil
		}
		// Skipped events are processed, so we can reset the counter
		skippedEvents = 0
	}

	// Case [canonicalBlock, pre-confirmed] || [pre-confirmed, pre-confirmed]
	return e.pendingEvents(matchedEvents, startBlock, skippedEvents, chunkSize)
}

func (e *EventFilter) canonicalEvents(
	matchedEvents []FilteredEvent,
	fromBlock,
	toBlock,
	skippedEvents,
	chunkSize uint64,
) ([]FilteredEvent, ContinuationToken, error) {
	matchedBlockIter, err := e.cachedFilters.NewMatchedBlockIterator(
		fromBlock,
		toBlock,
		uint64(e.maxScanned),
		&e.matcher,
		e.runningFilter,
	)
	if err != nil {
		return nil, ContinuationToken{}, err
	}

	var lastProccessedBlock uint64
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
			return nil, ContinuationToken{}, err
		}

		lastProccessedBlock = curBlock

		var header *core.Header
		header, err = core.GetBlockHeaderByNumber(e.txn, curBlock)
		if err != nil {
			return nil, ContinuationToken{}, err
		}

		var receipts []*core.TransactionReceipt
		receipts, err = core.GetReceiptsByBlockNum(e.txn, header.Number)
		if err != nil {
			return nil, ContinuationToken{}, err
		}

		var processedEvents uint64
		matchedEvents, processedEvents, err = e.matcher.AppendBlockEvents(
			matchedEvents,
			header,
			receipts,
			skippedEvents,
			chunkSize,
			false,
		)
		if err != nil {
			// Max events to scan exhausted mid block, continue from next unprocessed event
			if errors.Is(err, errChunkSizeReached) {
				cToken := ContinuationToken{fromBlock: curBlock, processedEvents: processedEvents}
				return matchedEvents, cToken, nil
			}
			return nil, ContinuationToken{}, err
		}

		// Skipped events are processed, so we can reset the counter
		skippedEvents = 0
	}

	// If max scans exhausted end of block
	if matchedBlockIter.scannedCount > matchedBlockIter.maxScanned && lastProccessedBlock <= e.toBlock {
		cToken := ContinuationToken{fromBlock: lastProccessedBlock, processedEvents: 0}
		return matchedEvents, cToken, nil
	}

	return matchedEvents, ContinuationToken{}, nil
}

// pendingEvents processes pending events
// Returns events from pre-latest block and pre-confirmed block
// Support access to pre-confirmed block in isolation when fromBlock > preLatest.Block.Number
func (e *EventFilter) pendingEvents(
	matchedEvents []FilteredEvent,
	fromBlock,
	skippedEvents,
	chunkSize uint64,
) ([]FilteredEvent, ContinuationToken, error) {
	pendingData, err := e.pendingDataFn()
	if err != nil {
		if errors.Is(err, core.ErrPendingDataNotFound) {
			return matchedEvents, ContinuationToken{}, nil
		}
		return nil, ContinuationToken{}, err
	}

	preLatest := pendingData.GetPreLatest()
	if preLatest != nil && fromBlock <= preLatest.Block.Number {
		var cToken ContinuationToken
		var err error
		matchedEvents, cToken, err = e.processPreLatestBlock(
			matchedEvents,
			preLatest.Block,
			skippedEvents,
			chunkSize,
		)
		if err != nil {
			return nil, ContinuationToken{}, err
		}

		if !cToken.IsEmpty() {
			return matchedEvents, cToken, nil
		}
		skippedEvents = 0
	}
	// Process pre-confirmed block
	return e.processPreConfirmedBlock(matchedEvents, pendingData, skippedEvents, chunkSize)
}

// processPreLatestBlock processes pre-latest block events
func (e *EventFilter) processPreLatestBlock(
	matchedEvents []FilteredEvent,
	preLatest *core.Block,
	skippedEvents,
	chunkSize uint64,
) ([]FilteredEvent, ContinuationToken, error) {
	if !e.matcher.TestBloom(preLatest.EventsBloom) {
		return matchedEvents, ContinuationToken{}, nil
	}

	var processedEvents uint64
	var err error
	matchedEvents, processedEvents, err = e.matcher.AppendBlockEvents(
		matchedEvents,
		preLatest.Header,
		preLatest.Receipts,
		skippedEvents,
		chunkSize,
		true,
	)
	if err != nil {
		if errors.Is(err, errChunkSizeReached) {
			cToken := ContinuationToken{
				fromBlock:       preLatest.Number,
				processedEvents: processedEvents,
			}
			return matchedEvents, cToken, nil
		}
		return nil, ContinuationToken{}, err
	}

	return matchedEvents, ContinuationToken{}, nil
}

// processPreConfirmedBlock processes pre-confirmed block events
func (e *EventFilter) processPreConfirmedBlock(
	matchedEvents []FilteredEvent,
	pendingData core.PendingData,
	skippedEvents,
	chunkSize uint64,
) ([]FilteredEvent, ContinuationToken, error) {
	pendingHeader := pendingData.GetHeader()
	if !e.matcher.TestBloom(pendingHeader.EventsBloom) {
		return matchedEvents, ContinuationToken{}, nil
	}

	var processedEvents uint64
	var err error
	matchedEvents, processedEvents, err = e.matcher.AppendBlockEvents(
		matchedEvents,
		pendingHeader,
		pendingData.GetBlock().Receipts,
		skippedEvents,
		chunkSize,
		false,
	)
	if err != nil {
		if errors.Is(err, errChunkSizeReached) {
			cToken := ContinuationToken{
				fromBlock:       pendingData.GetBlock().Number,
				processedEvents: processedEvents,
			}
			return matchedEvents, cToken, nil
		}
		return nil, ContinuationToken{}, err
	}

	return matchedEvents, ContinuationToken{}, nil
}
