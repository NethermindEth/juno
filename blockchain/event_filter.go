package blockchain

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/pruner"
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
	database       db.KeyValueStore
	fromBlock      uint64
	toBlock        uint64
	matcher        EventMatcher
	maxScanned     uint // maximum number of scanned blocks in single call.
	preConfirmedFn func() (PreConfirmedReader, error)
	cachedFilters  *AggregatedBloomFilterCache
	runningFilter  *core.RunningEventFilter
}

type EventFilterRange uint

const (
	EventFilterFrom EventFilterRange = iota
	EventFilterTo
)

func newEventFilter(
	database db.KeyValueStore,
	contractAddresses []felt.Address,
	keys [][]felt.Felt,
	fromBlock, toBlock uint64,
	preConfirmedFn func() (PreConfirmedReader, error),
	cachedFilters *AggregatedBloomFilterCache,
	runningFilter *core.RunningEventFilter,
) *EventFilter {
	return &EventFilter{
		database:       database,
		matcher:        NewEventMatcher(contractAddresses, keys),
		fromBlock:      fromBlock,
		toBlock:        toBlock,
		maxScanned:     math.MaxUint,
		preConfirmedFn: preConfirmedFn,
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
	header, err := core.GetBlockHeaderByHash(e.database, blockHash)
	if err != nil {
		return err
	}
	return e.SetRangeEndBlockByNumber(filterRange, header.Number)
}

// SetRangeEndBlockToL1Head sets an end of the block range to latest `l1_accepted` block
func (e *EventFilter) SetRangeEndBlockToL1Head(filterRange EventFilterRange) error {
	l1Head, err := core.GetL1Head(e.database)
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

	latest, err := core.GetChainHeight(e.database)
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

	// Reject queries whose canonical start block has been pruned. Skipped
	// when startBlock is in the pre-confirmed range (> latest), where
	// retention semantics don't apply.
	if startBlock <= latest {
		if err := pruner.RequireRetained(e.database, startBlock); err != nil {
			return nil, ContinuationToken{}, err
		}
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
		header, err = core.GetBlockHeaderByNumber(e.database, curBlock)
		if err != nil {
			return nil, ContinuationToken{}, err
		}

		var receipts []*core.TransactionReceipt
		receipts, err = core.GetReceiptsByBlockNumber(e.database, header.Number)
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

// pendingEvents processes pending events across every pre-confirmed block in
// the chain (head+1 .. tip), oldest-first. fromBlock and the continuation
// token's fromBlock select where to resume; the token's processedEvents
// counter applies to the resume block only.
func (e *EventFilter) pendingEvents(
	matchedEvents []FilteredEvent,
	fromBlock,
	skippedEvents,
	chunkSize uint64,
) ([]FilteredEvent, ContinuationToken, error) {
	preConfirmed, err := e.preConfirmedFn()
	if err != nil {
		if errors.Is(err, pending.ErrPreConfirmedNotFound) {
			return matchedEvents, ContinuationToken{}, nil
		}
		return nil, ContinuationToken{}, err
	}
	if preConfirmed == nil || preConfirmed.Length() == 0 {
		return matchedEvents, ContinuationToken{}, nil
	}

	// fromBlock = ^uint64(0) is the sentinel for "BlockID = preConfirmed". The
	// pre_confirmed tag refers to the single most recent block, so pin fromBlock
	// to the tip and let the per-block skip below drop the rest of the chain.
	if fromBlock == ^uint64(0) {
		fromBlock = preConfirmed.Head().Block.Number
	}

	for entry := range preConfirmed.OldestFirst() {
		blockNumber := entry.Block.Number
		if blockNumber < fromBlock {
			continue
		}
		if blockNumber > e.toBlock {
			break
		}

		header := entry.GetHeader()
		if !e.matcher.TestBloom(header.EventsBloom) {
			// Skipped events are scoped to the resume block; once we step past
			// it, reset so later blocks aren't under-counted.
			skippedEvents = 0
			continue
		}

		var processedEvents uint64
		matchedEvents, processedEvents, err = e.matcher.AppendBlockEvents(
			matchedEvents,
			header,
			entry.Block.Receipts,
			skippedEvents,
			chunkSize,
		)
		if err != nil {
			if errors.Is(err, errChunkSizeReached) {
				cToken := ContinuationToken{
					fromBlock:       blockNumber,
					processedEvents: processedEvents,
				}
				return matchedEvents, cToken, nil
			}
			return nil, ContinuationToken{}, err
		}
		skippedEvents = 0
	}

	return matchedEvents, ContinuationToken{}, nil
}
