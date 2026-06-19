package node_test

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockingCompiler is a stub compiler.Compiler whose Compile blocks until
// waitOn receives, letting the test hold compilation slots open.
type blockingCompiler struct {
	waitOn chan struct{}
}

func (b *blockingCompiler) Compile(
	_ context.Context, _ *starknet.SierraClass,
) (*starknet.CasmClass, error) {
	<-b.waitOn
	return &starknet.CasmClass{}, nil
}

func TestThrottledCompiler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		waitOn := make(chan struct{})
		// Concurrency budget of 2 and a max queue of 2.
		throttled := node.NewThrottledCompiler(&blockingCompiler{waitOn: waitOn}, 2, 2)

		var wg sync.WaitGroup
		// compile spawns a Compile call, then waits for the bubble to settle so
		// the throttler's running/queued counts are observed deterministically.
		compile := func(wantRunning, wantQueued int) {
			wg.Go(func() {
				_, err := throttled.Compile(t.Context(), &starknet.SierraClass{})
				assert.NoError(t, err) // assert, not require: runs off the test goroutine
			})
			synctest.Wait() // block until the spawned goroutine is durably blocked
			assert.Equal(t, wantRunning, throttled.JobsRunning())
			assert.Equal(t, wantQueued, throttled.QueueLen())
		}

		compile(1, 0) // occupies a slot
		compile(2, 0) // occupies the other slot
		compile(2, 1) // queued
		compile(2, 2) // queued

		// The queue is full, so the next request is rejected.
		_, err := throttled.Compile(t.Context(), &starknet.SierraClass{})
		require.ErrorIs(t, err, utils.ErrResourceBusy)

		// Release the four running/queued jobs and let them finish.
		for range 4 {
			waitOn <- struct{}{}
		}
		wg.Wait()
	})
}
