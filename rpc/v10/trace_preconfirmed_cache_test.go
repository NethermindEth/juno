package rpcv10

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/stretchr/testify/require"
)

// Done is evaluated only after a caller joins an existing flight. Observing it
// lets tests release the owner without sleeps or scheduler-dependent polling.
type traceWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *traceWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

type preConfirmedTestResult struct {
	trace  TransactionTrace
	header http.Header
	err    *jsonrpc.Error
}

func startTraceWaiter(t testing.TB, cache *preConfirmedTraceCache, index uint,
	ctx context.Context,
) <-chan preConfirmedTestResult {
	t.Helper()
	waiting := &traceWaitContext{Context: ctx, waiting: make(chan struct{})}
	result := make(chan preConfirmedTestResult, 1)
	go func() {
		trace, header, err := cache.getOrExecute(waiting, index,
			func() (TransactionTrace, http.Header, *jsonrpc.Error) {
				panic("waiter unexpectedly became execution owner")
			})
		result <- preConfirmedTestResult{trace, header, err}
	}()
	<-waiting.waiting
	return result
}

func TestPreConfirmedTraceCacheConcurrentRequests(t *testing.T) {
	cache := newPreConfirmedTraceCache(&preConfirmedTraceContext{})
	index := uint(1)
	entered, release := make(chan struct{}), make(chan struct{})
	owner := make(chan preConfirmedTestResult, 1)
	expected := TransactionTrace{Type: TxnInvoke}
	var executions atomic.Uint64
	go func() {
		trace, header, err := cache.getOrExecute(t.Context(), index,
			func() (TransactionTrace, http.Header, *jsonrpc.Error) {
				executions.Add(1)
				close(entered)
				<-release
				header := defaultExecutionHeader()
				header.Set(ExecutionStepsHeader, "123")
				return expected, header, nil
			})
		owner <- preConfirmedTestResult{trace, header, err}
	}()
	<-entered
	ctx, cancel := context.WithCancel(t.Context())
	canceled := startTraceWaiter(t, cache, index, ctx)
	cancel()
	require.NotNil(t, (<-canceled).err)
	waiters := make([]<-chan preConfirmedTestResult, 0, 17)
	for range 16 {
		waiters = append(waiters, startTraceWaiter(t, cache, index, t.Context()))
	}
	// Other transactions complete while the first owner is blocked.
	complete := func() (TransactionTrace, http.Header, *jsonrpc.Error) {
		return TransactionTrace{Type: TxnDeclare}, defaultExecutionHeader(), nil
	}
	for index := uint(2); index <= 130; index++ {
		_, _, err := cache.getOrExecute(t.Context(), index,
			complete)
		require.Nil(t, err)
	}
	waiters = append(waiters, startTraceWaiter(t, cache, index, t.Context()))
	close(release)
	result := <-owner
	require.Nil(t, result.err)
	require.Equal(t, "123", result.header.Get(ExecutionStepsHeader))
	trace, header, err := cache.getOrExecute(ctx, index, nil)
	require.Nil(t, err)
	require.Equal(t, expected, trace)
	require.Equal(t, "0", header.Get(ExecutionStepsHeader))
	require.Len(t, cache.entries, 130)
	for _, waiter := range waiters {
		result := <-waiter
		require.Nil(t, result.err)
		require.Equal(t, expected, result.trace)
		require.Equal(t, "0", result.header.Get(ExecutionStepsHeader))
	}
	require.Equal(t, uint64(1), executions.Load())
}

func TestPreConfirmedTraceCacheFailureAndPanic(t *testing.T) {
	for _, panics := range []bool{false, true} {
		name := "error"
		if panics {
			name = "panic"
		}
		t.Run(name, func(t *testing.T) {
			cache := newPreConfirmedTraceCache(&preConfirmedTraceContext{})
			index := uint(1)
			entered, release := make(chan struct{}), make(chan struct{})
			owner := make(chan any, 1)
			failure := rpccore.ErrUnexpectedError.CloneWithData("execution failed")
			go func() {
				defer func() { owner <- recover() }()
				_, _, _ = cache.getOrExecute(t.Context(), index,
					func() (TransactionTrace, http.Header, *jsonrpc.Error) {
						close(entered)
						<-release
						if panics {
							panic("VM panic")
						}
						return TransactionTrace{}, defaultExecutionHeader(), failure
					})
			}()
			<-entered
			first := startTraceWaiter(t, cache, index, t.Context())
			second := startTraceWaiter(t, cache, index, t.Context())
			close(release)
			recovered := <-owner
			if panics {
				require.Equal(t, "VM panic", recovered)
			} else {
				require.Nil(t, recovered)
			}
			one, two := <-first, <-second
			require.NotNil(t, one.err)
			require.Same(t, one.err, two.err)
			if !panics {
				require.Same(t, failure, one.err)
			}
			require.Empty(t, cache.entries)
			retried := false
			_, _, err := cache.getOrExecute(t.Context(), index,
				func() (TransactionTrace, http.Header, *jsonrpc.Error) {
					retried = true
					return TransactionTrace{Type: TxnInvoke}, defaultExecutionHeader(), nil
				})
			require.Nil(t, err)
			require.True(t, retried)
			require.Len(t, cache.entries, 1)
		})
	}
}
