package node

import (
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/utils/log"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/require"
)

// fakeService lets each test drive what Run does.
type fakeService struct {
	run func(ctx context.Context) error
}

func (f *fakeService) Run(ctx context.Context) error { return f.run(ctx) }

// TestStartService covers the three ways a service's Run can return and the
// expected effect on the node-wide context:
//   - error  -> shut the node down
//   - panic  -> shut the node down and propagate the panic
//   - nil    -> let the service exit without taking the node down
func TestStartService(t *testing.T) {
	newNode := func() *Node { return &Node{logger: log.NewNopZapLogger()} }

	t.Run("error shuts the node down", func(t *testing.T) {
		n := newNode()
		wg := conc.NewWaitGroup()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		failing := &fakeService{run: func(context.Context) error {
			return errors.New("boom")
		}}
		n.StartService(wg, ctx, cancel, failing)
		wg.Wait()

		require.Error(t, ctx.Err(), "a service error should shut the node down")
	})

	t.Run("panic shuts the node down and propagates", func(t *testing.T) {
		n := newNode()
		wg := conc.NewWaitGroup()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		panicking := &fakeService{run: func(context.Context) error {
			panic("boom")
		}}
		n.StartService(wg, ctx, cancel, panicking)

		require.Panics(t, func() { wg.Wait() }, "a panicking service should propagate the panic")
		require.Error(t, ctx.Err(), "a panicking service should shut the node down")
	})

	t.Run("clean return keeps the node running", func(t *testing.T) {
		n := newNode()
		wg := conc.NewWaitGroup()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		// Short-lived service: finishes its work and returns nil.
		exited := make(chan struct{})
		shortLived := &fakeService{run: func(context.Context) error {
			close(exited)
			return nil
		}}
		// Long-running service: lives until the node is shut down.
		stopped := make(chan struct{})
		longRunning := &fakeService{run: func(ctx context.Context) error {
			<-ctx.Done()
			close(stopped)
			return nil
		}}

		n.StartService(wg, ctx, cancel, longRunning)
		n.StartService(wg, ctx, cancel, shortLived)

		<-exited // the short-lived service has returned cleanly

		// Its clean exit must not cancel the node-wide context...
		require.NoError(t, ctx.Err(), "a clean service exit should not shut the node down")
		// ...and the long-running service must still be running.
		select {
		case <-stopped:
			t.Fatal("long-running service stopped: node was shut down unexpectedly")
		default:
		}

		// A real shutdown still stops everything.
		cancel()
		wg.Wait()
		<-stopped
	})
}
