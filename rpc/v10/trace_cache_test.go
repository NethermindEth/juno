package rpcv10

import (
	"testing"
	"time"

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
	baseHash := felt.FromUint64[felt.Felt](1)
	suffixHash := felt.FromUint64[felt.Felt](2)
	address := felt.FromUint64[felt.Address](3)
	key := felt.FromUint64[felt.Felt](4)
	baseValue := felt.FromUint64[felt.Felt](5)
	record := &blockTraceRecord{
		traces: []TracedBlockTransaction{{TransactionHash: &baseHash}},
		initialReads: &InitialReads{Storage: []StorageEntry{{
			ContractAddress: address, Key: key, Value: baseValue,
		}}},
	}
	extension := TraceBlockTransactionsResponse{
		Traces: []TracedBlockTransaction{{TransactionHash: &suffixHash}},
		InitialReads: &InitialReads{Storage: []StorageEntry{{
			ContractAddress: address,
			Key:             felt.FromUint64[felt.Felt](6),
			Value:           felt.FromUint64[felt.Felt](7),
		}}},
	}

	partial := record.view().response()
	require.Nil(t, partial.InitialReads, "initial reads describe block pre-state only when complete")
	require.Equal(t, len(partial.Traces), cap(partial.Traces))

	response := record.append(extension, 2).response()
	require.Len(t, response.Traces, 2)
	require.NotNil(t, response.InitialReads)
	require.Len(t, response.InitialReads.Storage, 2)
	require.Same(t, &baseHash, record.traces[0].TransactionHash)
	require.Equal(t, uint64(5), record.initialReads.Storage[0].Value.Uint64())
	require.True(t, record.complete)
	require.Len(t, partial.Traces, 1, "a previously returned prefix must not grow")
}

func TestBlockTraceCacheReadsOtherRecordWhileOneIsLocked(t *testing.T) {
	cache := newBlockTraceCache(2)
	blockedHash := felt.FromUint64[felt.Felt](1)
	otherHash := felt.FromUint64[felt.Felt](2)
	completeResponse := func() TraceBlockTransactionsResponse {
		return TraceBlockTransactionsResponse{
			Traces:       make([]TracedBlockTransaction, 1),
			InitialReads: emptyInitialReads(),
		}
	}
	cache.storeComplete(blockedHash, completeResponse())
	cache.storeComplete(otherHash, completeResponse())

	blockedRecord, found, _ := blockTraceCacheState(cache, blockedHash)
	require.True(t, found)
	blockedRecord.mu.Lock()
	defer blockedRecord.mu.Unlock()

	otherDone := make(chan struct{})
	go func() {
		defer close(otherDone)
		_, _ = cache.traceAt(otherHash, 0)
	}()
	select {
	case <-otherDone:
	case <-time.After(time.Second):
		t.Fatal("one record lock must not block another block's cache read")
	}
}

func TestBlockTraceCacheLookupTransitions(t *testing.T) {
	cache := newBlockTraceCache(1)
	blockHash := felt.FromUint64[felt.Felt](1)
	firstHash := felt.FromUint64[felt.Felt](2)
	secondHash := felt.FromUint64[felt.Felt](3)

	first := cache.lookupOrStart(blockHash, 0)
	require.Equal(t, traceCacheExtend, first.kind)

	for _, target := range []uint64{0, 1} {
		waiting := cache.lookupOrStart(blockHash, target)
		require.Equal(t, traceCacheWait, waiting.kind)
		require.True(t, first.work.flight.done == waiting.done)
	}

	prefix := first.work.commit(TraceBlockTransactionsResponse{
		Traces:       []TracedBlockTransaction{{TransactionHash: &firstHash}},
		InitialReads: &InitialReads{},
	}, 2)
	require.Len(t, prefix.Traces, 1)
	require.Nil(t, prefix.InitialReads)
	select {
	case <-first.work.flight.done:
	default:
		t.Fatal("commit must wake flight waiters")
	}

	hit := cache.lookupOrStart(blockHash, 0)
	require.Equal(t, traceCacheHit, hit.kind)
	require.Len(t, hit.response.Traces, 1)

	second := cache.lookupOrStart(blockHash, 1)
	require.Equal(t, traceCacheExtend, second.kind)
	require.Len(t, second.work.prefix(), 1)
	complete := second.work.commit(TraceBlockTransactionsResponse{
		Traces:       []TracedBlockTransaction{{TransactionHash: &secondHash}},
		InitialReads: &InitialReads{},
	}, 2)
	require.Len(t, complete.Traces, 2)
	require.NotNil(t, complete.InitialReads)
}
