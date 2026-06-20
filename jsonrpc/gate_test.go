package jsonrpc_test

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gate := jsonrpc.NewGate(2, 2) // up to 2 running + 2 queued
		ctx := t.Context()
		waitOn := make(chan struct{})

		var wg sync.WaitGroup
		// acquire spawns a goroutine that takes a slot, blocks until released,
		// then releases. synctest.Wait lets us observe the gate state once every
		// goroutine is durably blocked.
		acquire := func() {
			wg.Go(func() {
				// assert, not require: runs off the test goroutine
				assert.NoError(t, gate.Acquire(ctx))
				<-waitOn
				gate.Release()
			})
			synctest.Wait()
		}

		acquire()
		assert.Equal(t, 1, gate.Running())
		assert.Equal(t, 0, gate.Queued())
		acquire()
		assert.Equal(t, 2, gate.Running())
		assert.Equal(t, 0, gate.Queued())

		acquire() // queued: concurrency budget is full
		assert.Equal(t, 2, gate.Running())
		assert.Equal(t, 1, gate.Queued())
		acquire() // queued
		assert.Equal(t, 2, gate.Queued())

		// 2 running + 2 queued == capacity, so the next call is rejected at once.
		require.ErrorIs(t, gate.Acquire(ctx), jsonrpc.ErrServerBusy)

		// Release one running slot; a queued request is promoted to running.
		waitOn <- struct{}{}
		synctest.Wait()
		assert.Equal(t, 2, gate.Running())
		assert.Equal(t, 1, gate.Queued())

		// Drain the remaining goroutines.
		waitOn <- struct{}{}
		waitOn <- struct{}{}
		waitOn <- struct{}{}
		wg.Wait()
		assert.Equal(t, 0, gate.Running())
		assert.Equal(t, 0, gate.Queued())
	})
}

func TestGateContextCancellationWhileQueued(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// 1 running slot with a generous queue, so the second call is queued
		// (waiting for the slot) rather than rejected.
		gate := jsonrpc.NewGate(1, 10)
		waitOn := make(chan struct{})

		var wg sync.WaitGroup
		wg.Go(func() {
			assert.NoError(t, gate.Acquire(t.Context()))
			<-waitOn
			gate.Release()
		})
		synctest.Wait()
		assert.Equal(t, 1, gate.Running())

		// Second call blocks waiting for the slot; cancel its context while queued.
		ctx, cancel := context.WithCancel(t.Context())
		errCh := make(chan error, 1)
		go func() {
			errCh <- gate.Acquire(ctx)
		}()
		synctest.Wait()
		assert.Equal(t, 1, gate.Queued())

		cancel()
		require.ErrorIs(t, <-errCh, context.Canceled)
		synctest.Wait()
		assert.Equal(t, 0, gate.Queued()) // queue count is released on cancellation

		waitOn <- struct{}{}
		wg.Wait()
	})
}

func TestGateAlreadyCancelledContext(t *testing.T) {
	gate := jsonrpc.NewGate(1, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancelled before Acquire is called

	require.ErrorIs(t, gate.Acquire(ctx), context.Canceled)
	assert.Equal(t, 0, gate.Running()) // no slot was taken
	assert.Equal(t, 0, gate.Queued())
}

func TestGateZeroQueue(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// No queue: a single request runs, anything beyond is rejected at once.
		gate := jsonrpc.NewGate(1, 0)
		waitOn := make(chan struct{})

		var wg sync.WaitGroup
		wg.Go(func() {
			assert.NoError(t, gate.Acquire(t.Context()))
			<-waitOn
			gate.Release()
		})
		synctest.Wait()
		assert.Equal(t, 1, gate.Running())

		require.ErrorIs(t, gate.Acquire(t.Context()), jsonrpc.ErrServerBusy)
		assert.Equal(t, 0, gate.Queued())

		waitOn <- struct{}{}
		wg.Wait()
	})
}
