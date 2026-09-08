package rpcv10

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

func blockTraceCacheState(
	cache *blockTraceCache,
	blockHash felt.Felt,
) (*blockTraceRecord, bool, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	record, found := cache.records.Get(blockHash)
	_, inflight := cache.flights[blockHash]
	return record, found, inflight
}

func TestBlockTraceRecordAppendPreservesPublishedPrefix(t *testing.T) {
	cache := newBlockTraceCache(1)
	blockHash := felt.FromUint64[felt.Felt](1)
	secondHash := felt.FromUint64[felt.Felt](2)
	// Spare capacity exercises appending to a shared backing array.
	traces := make([]TracedBlockTransaction, 1, 4)
	traces[0].TransactionHash = &blockHash
	original := &blockTraceRecord{traces: traces}
	cache.records.Add(blockHash, original)
	partial := original.response()
	require.Equal(t, len(partial.Traces), cap(partial.Traces))

	work := cache.lookupOrStart(blockHash, 1, false).work
	response := work.commit(TraceBlockTransactionsResponse{
		Traces: []TracedBlockTransaction{{TransactionHash: &secondHash}},
	}, 2)
	require.Len(t, response.Traces, 2)
	require.Len(t, partial.Traces, 1)
	require.Len(t, original.traces, 1)
	require.Equal(t, blockHash, *partial.Traces[0].TransactionHash)
	require.Equal(t, secondHash, *response.Traces[1].TransactionHash)
	require.False(t, original.complete)
	published, _, _ := blockTraceCacheState(cache, blockHash)
	require.True(t, published.complete)
}

func TestBlockTraceCacheLookupTransitions(t *testing.T) {
	cache := newBlockTraceCache(1)
	blockHash := felt.FromUint64[felt.Felt](1)
	firstHash := felt.FromUint64[felt.Felt](2)
	secondHash := felt.FromUint64[felt.Felt](3)

	first := cache.lookupOrStart(blockHash, 0, false)
	require.Equal(t, traceCacheExecute, first.kind)

	for _, target := range []uint64{0, 1} {
		waiting := cache.lookupOrStart(blockHash, target, false)
		require.Equal(t, traceCacheWait, waiting.kind)
		require.True(t, first.work.flight == waiting.done)
	}

	prefix := first.work.commit(TraceBlockTransactionsResponse{
		Traces: []TracedBlockTransaction{{TransactionHash: &firstHash}},
	}, 2)
	require.Len(t, prefix.Traces, 1)
	require.Nil(t, prefix.InitialReads)
	select {
	case <-first.work.flight:
	default:
		t.Fatal("commit must wake flight waiters")
	}

	hit := cache.lookupOrStart(blockHash, 0, false)
	require.Equal(t, traceCacheHit, hit.kind)
	require.Len(t, hit.response.Traces, 1)

	second := cache.lookupOrStart(blockHash, 1, false)
	require.Equal(t, traceCacheExecute, second.kind)
	require.Len(t, second.work.record.traces, 1)
	complete := second.work.commit(TraceBlockTransactionsResponse{
		Traces: []TracedBlockTransaction{{TransactionHash: &secondHash}},
	}, 2)
	require.Len(t, complete.Traces, 2)
	require.Nil(t, complete.InitialReads)

	withReads := cache.lookupOrStart(blockHash, 1, true)
	require.Equal(t, traceCacheExecute, withReads.kind)
	require.Empty(t, withReads.work.record.traces)
	require.Equal(t, traceCacheHit, cache.lookupOrStart(blockHash, 1, false).kind)
	withReads.work.abort()
	require.Equal(t, traceCacheHit, cache.lookupOrStart(blockHash, 1, false).kind)

	withReads = cache.lookupOrStart(blockHash, 1, true)
	complete = withReads.work.commit(TraceBlockTransactionsResponse{
		Traces: []TracedBlockTransaction{
			{TransactionHash: &firstHash},
			{TransactionHash: &secondHash},
		},
		InitialReads: emptyInitialReads(),
	}, 2)
	require.NotNil(t, complete.InitialReads)
	require.Equal(t, traceCacheHit, cache.lookupOrStart(blockHash, 1, true).kind)
}
