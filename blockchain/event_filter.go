package blockchain

import (
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
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
	// txEventsBuf holds the pre-confirmed receipt projection. One filter serves one
	// request from one goroutine. Therefore the code can fill this buffer again for
	// each chain entry and each call. The projection then makes one allocation for
	// each filter, and not one allocation for each pre-confirmed block.
	txEventsBuf []core.TransactionEvents
}

type EventFilterRange uint

const (
	EventFilterFrom EventFilterRange = iota
	EventFilterTo
)

// PreConfirmedFilterSentinel marks a filter block bound as the pre_confirmed tag.
const PreConfirmedFilterSentinel uint64 = math.MaxUint64

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
	blockNum, err := core.GetBlockHeaderNumberByHash(e.database, blockHash)
	if err != nil {
		return err
	}
	return e.SetRangeEndBlockByNumber(filterRange, blockNum)
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
	BlockNumber      *uint64
	BlockHash        *felt.Felt
	TransactionHash  *felt.Felt
	TransactionIndex uint
	EventIndex       uint
}

func (e *EventFilter) Events(
	cToken *ContinuationToken,
	chunkSize uint64,
) ([]FilteredEvent, ContinuationToken, error) {
	latest, err := core.GetChainHeight(e.database)
	if err != nil {
		return nil, ContinuationToken{}, err
	}

	// Get pre_confirmed only if the range includes blocks above the canonical head.
	// Then read latest a second time. The first value can be too old, because the
	// node can commit blocks between the two reads. The canonical range stops at the
	// old value, and the pre_confirmed snapshot starts above those blocks, so no
	// range includes them. The second read prevents this. It also keeps those blocks
	// in the canonical range, which is the only range that has a block hash. A fixed
	// head is not a solution, because SnapshotForBlock removes committed blocks and
	// an old head gives an empty view. If the fetch fails, use the canonical range.
	var preConfirmed PreConfirmedReader
	if e.toBlock > latest {
		if fetched, err := e.preConfirmedFn(); err == nil {
			preConfirmed = fetched
		}

		latest, err = core.GetChainHeight(e.database)
		if err != nil {
			return nil, ContinuationToken{}, err
		}
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

	var matchedEvents []FilteredEvent
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

	// Case [canonicalBlock, pre-confirmed] || [pre-confirmed, pre-confirmed].
	return e.preConfirmedEvents(
		matchedEvents,
		preConfirmed,
		startBlock,
		latest,
		skippedEvents,
		chunkSize,
	)
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
		curBlockNum, ok, err := matchedBlockIter.Next()
		if !ok {
			// iteration complete
			if err == nil {
				break
			}

			// If max scans exhausted end of block
			if errors.Is(err, ErrMaxScannedBlockLimitExceed) {
				// Next candidate block for continuation token
				lastProccessedBlock = curBlockNum
				break
			}
			return nil, ContinuationToken{}, err
		}

		lastProccessedBlock = curBlockNum

		blockEvents, err := core.GetTransactionEventsByBlockNumber(e.database, curBlockNum)
		if err != nil {
			return nil, ContinuationToken{}, err
		}

		var processedEvents uint64
		matchedEvents, processedEvents, err = e.matcher.AppendBlockEvents(
			matchedEvents,
			curBlockNum,
			func() (*felt.Felt, error) {
				return core.GetBlockHeaderHashByNumber(e.database, curBlockNum)
			},
			blockEvents,
			skippedEvents,
			chunkSize,
		)
		if err != nil {
			// Max events to scan exhausted mid block, continue from next unprocessed event
			if errors.Is(err, errChunkSizeReached) {
				cToken := ContinuationToken{fromBlock: curBlockNum, processedEvents: processedEvents}
				return matchedEvents, cToken, nil
			}
			return nil, ContinuationToken{}, err
		}

		// Skipped events are processed, so we can reset the counter
		skippedEvents = 0
	}

	// If max scans exhausted end of block
	maxScanReachedPrematurely := matchedBlockIter.scannedCount > matchedBlockIter.maxScanned &&
		lastProccessedBlock <= e.toBlock
	if maxScanReachedPrematurely {
		cToken := ContinuationToken{fromBlock: lastProccessedBlock, processedEvents: 0}
		return matchedEvents, cToken, nil
	}

	return matchedEvents, ContinuationToken{}, nil
}

// appendTransactionEvents adds the events subset of each receipt to dst. Both event
// sources then give the matcher the same type. The caller can use dst again for each
// block. This is safe, because the function copies only slice headers and pointers,
// and the matcher keeps no reference to dst.
func appendTransactionEvents(
	dst []core.TransactionEvents,
	receipts []*core.TransactionReceipt,
) []core.TransactionEvents {
	for _, receipt := range receipts {
		dst = append(dst, core.TransactionEvents{
			Events:          receipt.Events,
			TransactionHash: receipt.TransactionHash,
		})
	}
	return dst
}

// preConfirmedEvents processes pending events across every pre-confirmed block in
// the chain (head+1 .. tip), oldest-first. fromBlock and the continuation
// token's fromBlock select where to resume; the token's processedEvents
// counter applies to the resume block only.
func (e *EventFilter) preConfirmedEvents(
	matchedEvents []FilteredEvent,
	preConfirmed PreConfirmedReader,
	fromBlock,
	latest,
	skippedEvents,
	chunkSize uint64,
) ([]FilteredEvent, ContinuationToken, error) {
	if preConfirmed == nil || preConfirmed.Length() == 0 {
		return matchedEvents, ContinuationToken{}, nil
	}

	// fromBlock = PreConfirmedFilterSentinel is the sentinel for "BlockID =
	// preConfirmed". The pre_confirmed tag refers to the single most recent block,
	// so pin fromBlock to the tip and let the per-block skip below drop the rest of
	// the chain.
	if fromBlock == PreConfirmedFilterSentinel {
		fromBlock = preConfirmed.Head().Block.Number
	}

	var err error
	for entry := range preConfirmed.OldestFirst() {
		blockNumber := entry.Block.Number
		// Skip blocks the canonical scan already covered (numbers <= latest, an
		// overlap opened by a head advance mid-query) and blocks below the resume
		// point.
		if blockNumber <= latest || blockNumber < fromBlock {
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

		e.txEventsBuf = appendTransactionEvents(e.txEventsBuf[:0], entry.Block.Receipts)

		var processedEvents uint64
		matchedEvents, processedEvents, err = e.matcher.AppendBlockEvents(
			matchedEvents,
			blockNumber,
			func() (*felt.Felt, error) { return header.Hash, nil },
			e.txEventsBuf,
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
