package rpcv10

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/lru"
)

// blockTraceCache publishes immutable trace prefixes and permits one active execution per block.
// mu protects cache and flight membership; execution and trace appends happen outside the lock.
type blockTraceCache struct {
	mu      sync.Mutex
	records *lru.SimpleCache[felt.Felt, *blockTraceRecord]
	flights map[felt.Felt]chan struct{}
}

// Published records are immutable. The flight owner may append beyond the prefix's length,
// but must publish a new record. Responses borrow read-only data with capacity-limited slices.
type blockTraceRecord struct {
	traces       []TracedBlockTransaction
	initialReads *InitialReads // only complete records may contain initial reads
	complete     bool
}

type traceCacheLookupKind uint8

const (
	traceCacheHit traceCacheLookupKind = iota + 1
	traceCacheWait
	traceCacheExecute
)

type traceCacheLookup struct {
	kind     traceCacheLookupKind
	response TraceBlockTransactionsResponse
	done     <-chan struct{}
	work     *traceCacheWork
}

// Work retains its prefix across eviction and publishes only after successful execution.
type traceCacheWork struct {
	cache  *blockTraceCache
	hash   felt.Felt
	flight chan struct{}
	record *blockTraceRecord
}

func newBlockTraceCache(limit int) *blockTraceCache {
	return &blockTraceCache{
		records: lru.NewSimple[felt.Felt, *blockTraceRecord](limit),
		flights: make(map[felt.Felt]chan struct{}),
	}
}

func (c *blockTraceCache) completeResponse(
	blockHash *felt.Felt,
	requireInitialReads bool,
) (TraceBlockTransactionsResponse, bool) {
	record, found := c.record(blockHash)
	if !found || !record.complete || requireInitialReads && record.initialReads == nil {
		return TraceBlockTransactionsResponse{}, false
	}
	return record.response(), true
}

func (c *blockTraceCache) traceAt(
	blockHash *felt.Felt,
	index uint64,
) (TracedBlockTransaction, bool) {
	record, found := c.record(blockHash)
	if !found || index >= uint64(len(record.traces)) {
		return TracedBlockTransaction{}, false
	}
	return record.traces[index], true
}

func (c *blockTraceCache) lookupOrStart(
	blockHash felt.Felt,
	target uint64,
	requireInitialReads bool,
) traceCacheLookup {
	c.mu.Lock()
	defer c.mu.Unlock()
	record, _ := c.records.Get(blockHash)
	if record != nil && target < uint64(len(record.traces)) &&
		(!requireInitialReads || record.initialReads != nil) {
		return traceCacheLookup{kind: traceCacheHit, response: record.response()}
	}
	if flight, found := c.flights[blockHash]; found {
		return traceCacheLookup{kind: traceCacheWait, done: flight}
	}
	// Missing initial reads require a full replay. Keep the old record available until commit.
	if record == nil || requireInitialReads {
		record = &blockTraceRecord{}
	}
	flight := make(chan struct{})
	c.flights[blockHash] = flight
	return traceCacheLookup{
		kind: traceCacheExecute,
		work: &traceCacheWork{cache: c, hash: blockHash, flight: flight, record: record},
	}
}

// storeComplete takes ownership of the response containers. InitialReads must be non-nil.
func (c *blockTraceCache) storeComplete(
	blockHash *felt.Felt,
	response TraceBlockTransactionsResponse,
) {
	record := &blockTraceRecord{
		traces:       response.Traces,
		initialReads: response.InitialReads,
		complete:     true,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records.Add(*blockHash, record)
}

func (c *blockTraceCache) record(blockHash *felt.Felt) (*blockTraceRecord, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.records.Get(*blockHash)
}

func (c *blockTraceCache) finishLocked(blockHash *felt.Felt, flight chan struct{}) {
	delete(c.flights, *blockHash)
	close(flight)
}

func (r *blockTraceRecord) response() TraceBlockTransactionsResponse {
	return TraceBlockTransactionsResponse{
		Traces:       r.traces[:len(r.traces):len(r.traces)],
		InitialReads: r.initialReads,
	}
}

func (w *traceCacheWork) commit(
	executed TraceBlockTransactionsResponse,
	totalTransactions int,
) TraceBlockTransactionsResponse {
	record := &blockTraceRecord{
		traces:       append(w.record.traces, executed.Traces...),
		initialReads: executed.InitialReads,
	}
	record.complete = len(record.traces) == totalTransactions

	cache := w.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.records.Add(w.hash, record)
	cache.finishLocked(&w.hash, w.flight)
	return record.response()
}

func (w *traceCacheWork) abort() {
	cache := w.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.flights[w.hash] != w.flight {
		return
	}
	cache.finishLocked(&w.hash, w.flight)
}
