package node_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/node"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
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
	// newNode returns a node logging into an observer so tests can assert on the
	// messages StartService emits.
	newNode := func() (*node.Node, *observer.ObservedLogs) {
		core, logs := observer.New(zapcore.DebugLevel)
		return node.NewWithLogger(log.NewZapLoggerWithCore(core)), logs
	}

	t.Run("error shuts the node down", func(t *testing.T) {
		n, logs := newNode()
		wg := conc.NewWaitGroup()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		failing := &fakeService{run: func(context.Context) error {
			return errors.New("boom")
		}}
		n.StartService(wg, ctx, cancel, failing)
		wg.Wait()

		require.Error(t, ctx.Err(), "a service error should shut the node down")
		require.Equal(t, 1, logs.FilterMessage("Service error").Len(),
			"a service error should be logged")
	})

	t.Run("panic shuts the node down and propagates", func(t *testing.T) {
		n, _ := newNode()
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
		n, logs := newNode()
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

		// Its clean exit must not take the long-running service down with it.
		select {
		case <-stopped:
			t.Fatal("long-running service stopped: node was shut down unexpectedly")
		default:
		}
		// And the early return should be flagged as unexpected.
		require.Equal(t, 1, logs.FilterMessage("Service stopped before node shutdown was requested").Len(),
			"a clean exit before shutdown should be logged")

		// A real shutdown still stops everything.
		cancel()
		wg.Wait()
		<-stopped
	})
}
