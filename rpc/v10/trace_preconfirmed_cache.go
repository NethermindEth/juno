package rpcv10

import (
	"context"
	"net/http"
	"sync"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
)

type preConfirmedTraceContext struct {
	baseHash          felt.Felt
	generation        uint64
	revealedBlockHash felt.Felt
	hasRevealedBlock  bool
}

// An entry starts in flight and retains an immutable result on success.
type preConfirmedTraceEntry struct {
	done  chan struct{} // non-nil during execution; read under the cache mutex
	trace TransactionTrace
	err   *jsonrpc.Error
}

// Each cache belongs to one block context, where transaction indexes stay stable
// across append-only updates. Retired caches remain reachable only by existing callers.
type preConfirmedTraceCache struct {
	context preConfirmedTraceContext
	mu      sync.Mutex
	entries map[uint]*preConfirmedTraceEntry
}

func newPreConfirmedTraceCache(context *preConfirmedTraceContext) *preConfirmedTraceCache {
	return &preConfirmedTraceCache{
		context: *context,
		entries: make(map[uint]*preConfirmedTraceEntry),
	}
}

// getOrExecute elects one owner per transaction. The owner executes synchronously outside
// the mutex; cancellation only detaches waiters, matching the VM's current API.
func (c *preConfirmedTraceCache) getOrExecute(
	ctx context.Context,
	index uint,
	execute func() (TransactionTrace, http.Header, *jsonrpc.Error),
) (trace TransactionTrace, header http.Header, rpcErr *jsonrpc.Error) {
	c.mu.Lock()
	if entry, found := c.entries[index]; found {
		done := entry.done
		c.mu.Unlock()
		if done == nil {
			return entry.trace, defaultExecutionHeader(), entry.err
		}
		select {
		case <-ctx.Done():
			return TransactionTrace{}, defaultExecutionHeader(),
				rpccore.ErrUnexpectedError.CloneWithData(ctx.Err().Error())
		case <-done:
			return entry.trace, defaultExecutionHeader(), entry.err
		}
	}
	entry := &preConfirmedTraceEntry{done: make(chan struct{})}
	c.entries[index] = entry
	c.mu.Unlock()

	// Publish once on return or panic. A panic leaves this default error in place
	// and propagates to the owner's recovery; failed entries permit later retries.
	rpcErr = rpccore.ErrInternal
	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		entry.trace, entry.err = trace, rpcErr
		if rpcErr != nil {
			delete(c.entries, index)
		}
		close(entry.done)
		entry.done = nil
	}()
	return execute()
}
