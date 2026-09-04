package rpcv10

import (
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/lru"
)

// blockTraceCache retains successful trace progress in an LRU and permits one active extension per
// block. An extension becomes visible only after its entire requested suffix succeeds; failed work
// only wakes its waiters. mu protects LRU and flight membership, while per-record locks allow
// unrelated blocks to be read and extended concurrently.
type blockTraceCache struct {
	mu sync.Mutex

	records *lru.SimpleCache[felt.Felt, *blockTraceRecord]
	flights map[felt.Felt]*traceFlight
}

// blockTraceRecord is the append-only tracing progress for one block. Returned trace slices are
// capacity-limited, so a later extension cannot change their visible length or elements.
type blockTraceRecord struct {
	mu sync.RWMutex

	traces       []TracedBlockTransaction
	initialReads *InitialReads // non-nil once the record has been published
	complete     bool
}

type blockTraceRecordView struct {
	traces       []TracedBlockTransaction
	initialReads *InitialReads
	complete     bool
}

type traceFlight struct {
	done chan struct{}
}

type traceCacheLookupKind uint8

const (
	traceCacheHit traceCacheLookupKind = iota + 1
	traceCacheWait
	traceCacheExtend
)

type traceCacheLookup struct {
	kind     traceCacheLookupKind
	response TraceBlockTransactionsResponse
	done     <-chan struct{}
	work     *traceCacheWork
}

// traceCacheWork is owned by the caller executing a suffix. Work retains its record across LRU
// eviction; only a successful commit republishes it. After commit removes the flight, the deferred
// abort sees that it is no longer active and becomes a no-op.
type traceCacheWork struct {
	cache  *blockTraceCache
	hash   felt.Felt
	flight *traceFlight
	record *blockTraceRecord
}

func (r *blockTraceRecord) view() blockTraceRecordView {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewLocked()
}

func (r *blockTraceRecord) viewLocked() blockTraceRecordView {
	return blockTraceRecordView{
		traces:       r.traces[:len(r.traces):len(r.traces)],
		initialReads: r.initialReads,
		complete:     r.complete,
	}
}

func (r *blockTraceRecord) append(
	extension TraceBlockTransactionsResponse,
	totalTransactions int,
) blockTraceRecordView {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.traces = append(r.traces, extension.Traces...)
	r.initialReads = mergeInitialReads(r.initialReads, extension.InitialReads)
	r.complete = len(r.traces) == totalTransactions
	return r.viewLocked()
}

func (v blockTraceRecordView) response() TraceBlockTransactionsResponse {
	// Responses borrow record-owned data and must be treated as read-only.
	response := TraceBlockTransactionsResponse{Traces: v.traces}
	if v.complete {
		response.InitialReads = v.initialReads
	}
	return response
}

func (w *traceCacheWork) prefix() []TracedBlockTransaction {
	return w.record.view().traces
}

func (w *traceCacheWork) commit(
	extension TraceBlockTransactionsResponse,
	totalTransactions int,
) TraceBlockTransactionsResponse {
	view := w.record.append(extension, totalTransactions)

	cache := w.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.records.Add(w.hash, w.record)
	cache.finishLocked(w.hash, w.flight)
	return view.response()
}

func (w *traceCacheWork) abort() {
	cache := w.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.flights[w.hash] != w.flight {
		return
	}
	cache.finishLocked(w.hash, w.flight)
}

func newBlockTraceCache(limit int) *blockTraceCache {
	return &blockTraceCache{
		records: lru.NewSimple[felt.Felt, *blockTraceRecord](limit),
		flights: make(map[felt.Felt]*traceFlight),
	}
}

func (c *blockTraceCache) completeResponse(
	blockHash felt.Felt,
) (TraceBlockTransactionsResponse, bool) {
	record, found := c.record(blockHash)
	if !found {
		return TraceBlockTransactionsResponse{}, false
	}
	view := record.view()
	if !view.complete {
		return TraceBlockTransactionsResponse{}, false
	}
	return view.response(), true
}

func (c *blockTraceCache) traceAt(
	blockHash felt.Felt,
	index uint64,
) (TracedBlockTransaction, bool) {
	record, found := c.record(blockHash)
	if !found {
		return TracedBlockTransaction{}, false
	}
	view := record.view()
	if index >= uint64(len(view.traces)) {
		return TracedBlockTransaction{}, false
	}
	return view.traces[index], true
}

func (c *blockTraceCache) record(blockHash felt.Felt) (*blockTraceRecord, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.records.Get(blockHash)
}

func (c *blockTraceCache) lookupOrStart(
	blockHash felt.Felt,
	target uint64,
) traceCacheLookup {
	c.mu.Lock()
	record, _ := c.records.Get(blockHash)
	if flight, found := c.flights[blockHash]; found {
		c.mu.Unlock()
		if lookup, found := lookupBlockTraceRecord(record, target); found {
			return lookup
		}
		return traceCacheLookup{kind: traceCacheWait, done: flight.done}
	}
	defer c.mu.Unlock()

	if lookup, found := lookupBlockTraceRecord(record, target); found {
		return lookup
	}
	if record == nil {
		record = &blockTraceRecord{}
	}
	flight := &traceFlight{done: make(chan struct{})}
	c.flights[blockHash] = flight
	return traceCacheLookup{
		kind: traceCacheExtend,
		work: &traceCacheWork{cache: c, hash: blockHash, flight: flight, record: record},
	}
}

func lookupBlockTraceRecord(
	record *blockTraceRecord,
	target uint64,
) (traceCacheLookup, bool) {
	if record == nil {
		return traceCacheLookup{}, false
	}
	view := record.view()
	if target < uint64(len(view.traces)) {
		return traceCacheLookup{kind: traceCacheHit, response: view.response()}, true
	}
	return traceCacheLookup{}, false
}

// storeComplete takes ownership of the response containers. InitialReads must be non-nil.
func (c *blockTraceCache) storeComplete(
	blockHash felt.Felt,
	response TraceBlockTransactionsResponse,
) {
	record := &blockTraceRecord{
		traces:       response.Traces,
		initialReads: response.InitialReads,
		complete:     true,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records.Add(blockHash, record)
}

func (c *blockTraceCache) finishLocked(blockHash felt.Felt, flight *traceFlight) {
	delete(c.flights, blockHash)
	close(flight.done)
}
