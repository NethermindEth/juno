package node_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils/throttler"
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
	waitOn := make(chan struct{})
	// Concurrency budget of 2 and a max queue of 2.
	throttled := node.NewThrottledCompiler(&blockingCompiler{waitOn: waitOn}, 2, 2)

	var wg sync.WaitGroup
	compile := func() {
		wg.Go(func() {
			_, err := throttled.Compile(t.Context(), &starknet.SierraClass{})
			require.NoError(t, err)
		})
		time.Sleep(time.Millisecond)
	}

	compile() // occupies a slot
	assert.Equal(t, 0, throttled.QueueLen())
	compile() // occupies the other slot
	assert.Equal(t, 0, throttled.QueueLen())

	compile() // should be queued
	assert.Equal(t, 1, throttled.QueueLen())
	compile() // should be queued
	assert.Equal(t, 2, throttled.QueueLen())

	// The queue is full, so the next request is rejected.
	_, err := throttled.Compile(t.Context(), &starknet.SierraClass{})
	require.ErrorIs(t, err, throttler.ErrResourceBusy)

	// Release the four running/queued jobs and let them finish.
	for range 4 {
		waitOn <- struct{}{}
	}
	wg.Wait()
}
