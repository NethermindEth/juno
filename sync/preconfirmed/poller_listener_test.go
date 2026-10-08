package preconfirmed_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// listenerCall is one call a poller made on its listener.
type listenerCall struct {
	method string
	label  string
	took   time.Duration
	gap    uint64
}

func pollSucceeded(update string, took time.Duration) listenerCall {
	return listenerCall{method: "OnPollSucceeded", label: update, took: took}
}

func pollFailed(reason string) listenerCall {
	return listenerCall{method: "OnPollFailed", label: reason}
}

func backfillStarted(gap uint64) listenerCall {
	return listenerCall{method: "OnBackfill", gap: gap}
}

// listenerRecorder is a [preconfirmed.EventListener] that records its calls in order.
type listenerRecorder struct {
	mu    sync.Mutex
	calls []listenerCall
}

func (r *listenerRecorder) record(call listenerCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *listenerRecorder) OnPollSucceeded(update string, took time.Duration) {
	r.record(pollSucceeded(update, took))
}

func (r *listenerRecorder) OnPollFailed(reason string) {
	r.record(pollFailed(reason))
}

func (r *listenerRecorder) OnBackfill(gap uint64) {
	r.record(backfillStarted(gap))
}

// take returns the calls recorded since the previous take.
func (r *listenerRecorder) take() []listenerCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	calls := r.calls
	r.calls = nil
	return calls
}

// wirePollerWithListener is [wirePoller] with a chosen listener.
func wirePollerWithListener(
	t *testing.T,
	bc *blockchain.Blockchain,
	head *core.Header,
	ds preconfirmed.DataSource,
	listener preconfirmed.EventListener,
) harness {
	t.Helper()
	highest := &atomic.Pointer[core.Header]{}
	highest.Store(head)

	p := preconfirmed.NewPoller(
		ds, bc, highest, tickInterval, staleAfter, onDemandWait, listener, log.NewNopZapLogger(),
	)
	return harness{poller: p, highest: highest}
}

// reportedFailures are data source errors that fail a pre-confirmed poll, with the reason the
// poll reports for each.
var reportedFailures = []struct {
	name   string
	err    error
	reason string
}{
	{
		name:   "wire error",
		err:    errors.New("wire boom"),
		reason: preconfirmed.FailureError,
	},
	{
		name:   "rate limited",
		err:    fmt.Errorf("querying: %w", feeder.ErrRateLimited),
		reason: preconfirmed.FailureRateLimited,
	},
	{
		name:   "not found",
		err:    fmt.Errorf("querying: %w", feeder.ErrPreConfirmedBlockNotFound),
		reason: preconfirmed.FailureNotFound,
	},
}

// pollLatency is how long a delayed data source reply takes.
const pollLatency = 10 * time.Millisecond

// lateLatest delays a PreConfirmedBlockLatest reply by pollLatency.
func lateLatest(context.Context, string, uint64) { time.Sleep(pollLatency) }

// lateByNumber delays a PreConfirmedBlockByNumber reply by pollLatency.
func lateByNumber(context.Context, uint64, string, uint64) { time.Sleep(pollLatency) }

// A poll that applies the latest update reports the update's kind and how long the poll took,
// backfill included.
func TestPollerReportsSuccessfulPolls(t *testing.T) {
	t.Parallel()

	t.Run("each update kind with its duration", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockPreConfirmedDataSource(ctrl)
		gomock.InOrder(
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(makeTestPreConfirmedBlock("r0", 0), uint64(1), nil).
				Do(lateLatest),
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
				Return(makeTestDelta("r0", 2), uint64(1), nil).
				Do(lateLatest),
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(2)).
				Return(starknet.PreConfirmedNoChange{}, uint64(1), nil).
				Do(lateLatest),
		)

		synctest.Test(t, func(t *testing.T) {
			listener := &listenerRecorder{}
			h := wirePollerWithListener(t, fx.bc, fx.head, ds, listener)
			go h.poller.Run(t.Context())
			synctest.Wait()

			// Check half a tick after each tick, once its delayed reply has landed.
			time.Sleep(tickInterval / 2)
			for _, update := range []string{
				preconfirmed.UpdateFull, preconfirmed.UpdateDelta, preconfirmed.UpdateNoChange,
			} {
				time.Sleep(tickInterval)
				synctest.Wait()
				require.Equal(t, []listenerCall{pollSucceeded(update, pollLatency)}, listener.take())
			}
		})
	})

	t.Run("a backfill as it starts and the success after it", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockPreConfirmedDataSource(ctrl)
		gomock.InOrder(
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(makeTestPreConfirmedBlock("r0", 0), uint64(1), nil),
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(0)).
				Return(makeTestPreConfirmedBlock("r1", 0), uint64(2), nil).
				Do(lateLatest),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "r0", uint64(0)).
				Return(makeTestDelta("r0", 3), nil).
				Do(lateByNumber),
		)

		synctest.Test(t, func(t *testing.T) {
			listener := &listenerRecorder{}
			h := wirePollerWithListener(t, fx.bc, fx.head, ds, listener)
			go h.poller.Run(t.Context())
			synctest.Wait()

			// Tick 1 seeds block 1.
			time.Sleep(tickInterval)
			synctest.Wait()
			require.Equal(t,
				[]listenerCall{pollSucceeded(preconfirmed.UpdateFull, 0)},
				listener.take(),
			)

			// Tick 2 finds block 2, so it backfills block 1: midway through that request.
			time.Sleep(tickInterval + pollLatency + pollLatency/2)
			synctest.Wait()
			require.Equal(t, []listenerCall{backfillStarted(1)}, listener.take())

			// Block 1's reply lands, then block 2 is applied.
			time.Sleep(pollLatency)
			synctest.Wait()
			require.Equal(t,
				[]listenerCall{pollSucceeded(preconfirmed.UpdateFull, 2*pollLatency)},
				listener.take(),
			)
		})
	})
}

// A poll that backfills reports how many blocks the gap below the latest holds.
func TestPollerReportsBackfillGap(t *testing.T) {
	t.Parallel()

	t.Run("from the stored tip", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockPreConfirmedDataSource(ctrl)
		gomock.InOrder(
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(makeTestPreConfirmedBlock("r2", 0), uint64(2), nil),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "", uint64(0)).
				Return(makeTestPreConfirmedBlock("r1", 0), nil),
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r2", uint64(0)).
				Return(makeTestPreConfirmedBlock("r5", 0), uint64(5), nil),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(2), "r2", uint64(0)).
				Return(starknet.PreConfirmedNoChange{}, nil),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(3), "", uint64(0)).
				Return(makeTestPreConfirmedBlock("r3", 0), nil),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(4), "", uint64(0)).
				Return(makeTestPreConfirmedBlock("r4", 0), nil),
		)

		synctest.Test(t, func(t *testing.T) {
			listener := &listenerRecorder{}
			h := wirePollerWithListener(t, fx.bc, fx.head, ds, listener)
			go h.poller.Run(t.Context())
			synctest.Wait()

			// Tick 1 stores blocks 1 and 2.
			time.Sleep(tickInterval)
			synctest.Wait()
			require.Equal(t,
				[]listenerCall{backfillStarted(1), pollSucceeded(preconfirmed.UpdateFull, 0)},
				listener.take(),
			)

			// Tick 2 finds block 5, so blocks 2 to 4 are the gap.
			time.Sleep(tickInterval)
			synctest.Wait()
			require.Equal(t,
				[]listenerCall{backfillStarted(3), pollSucceeded(preconfirmed.UpdateFull, 0)},
				listener.take(),
			)
		})
	})

	t.Run("from above the head with nothing stored", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockPreConfirmedDataSource(ctrl)
		gomock.InOrder(
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(makeTestPreConfirmedBlock("r3", 0), uint64(3), nil),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "", uint64(0)).
				Return(makeTestPreConfirmedBlock("r1", 0), nil),
			ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(2), "", uint64(0)).
				Return(makeTestPreConfirmedBlock("r2", 0), nil),
		)

		synctest.Test(t, func(t *testing.T) {
			listener := &listenerRecorder{}
			h := wirePollerWithListener(t, fx.bc, fx.head, ds, listener)
			go h.poller.Run(t.Context())
			synctest.Wait()

			// The head is block 0, so tick 1 finds blocks 1 and 2 below block 3.
			time.Sleep(tickInterval)
			synctest.Wait()
			require.Equal(t,
				[]listenerCall{backfillStarted(2), pollSucceeded(preconfirmed.UpdateFull, 0)},
				listener.take(),
			)
		})
	})
}

// A poll that does not apply the latest update reports why.
func TestPollerReportsFailedPolls(t *testing.T) {
	t.Parallel()

	for _, tc := range reportedFailures {
		t.Run("latest "+tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newChainFixture(t)

			ctrl := gomock.NewController(t)
			ds := mocks.NewMockPreConfirmedDataSource(ctrl)
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(nil, uint64(0), tc.err)

			synctest.Test(t, func(t *testing.T) {
				listener := &listenerRecorder{}
				h := wirePollerWithListener(t, fx.bc, fx.head, ds, listener)
				go h.poller.Run(t.Context())
				synctest.Wait()

				time.Sleep(tickInterval)
				synctest.Wait()
				require.Equal(t, []listenerCall{pollFailed(tc.reason)}, listener.take())
			})
		})

		t.Run("backfill "+tc.name, func(t *testing.T) {
			t.Parallel()
			fx := newChainFixture(t)

			ctrl := gomock.NewController(t)
			ds := mocks.NewMockPreConfirmedDataSource(ctrl)
			gomock.InOrder(
				ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
					Return(makeTestPreConfirmedBlock("r2", 0), uint64(2), nil),
				ds.EXPECT().PreConfirmedBlockByNumber(gomock.Any(), uint64(1), "", uint64(0)).
					Return(nil, tc.err),
			)

			synctest.Test(t, func(t *testing.T) {
				listener := &listenerRecorder{}
				h := wirePollerWithListener(t, fx.bc, fx.head, ds, listener)
				go h.poller.Run(t.Context())
				synctest.Wait()

				time.Sleep(tickInterval)
				synctest.Wait()
				require.Equal(t,
					[]listenerCall{backfillStarted(1), pollFailed(tc.reason)},
					listener.take(),
				)
			})
		})
	}

	t.Run("apply error", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		unsupported := makeTestPreConfirmedBlock("r0", 0)
		unsupported.Version = "0.99.0"

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockPreConfirmedDataSource(ctrl)
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(unsupported, uint64(1), nil)

		synctest.Test(t, func(t *testing.T) {
			listener := &listenerRecorder{}
			h := wirePollerWithListener(t, fx.bc, fx.head, ds, listener)
			go h.poller.Run(t.Context())
			synctest.Wait()

			time.Sleep(tickInterval)
			synctest.Wait()
			require.Equal(t, []listenerCall{pollFailed(preconfirmed.FailureError)}, listener.take())
		})
	})
}

// A tick skipped because the node is not at the tip reports nothing.
func TestPollerReportsNothingWhileNotAtTip(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockPreConfirmedDataSource(ctrl)
	// No expectations set: any wire call fails the test.

	synctest.Test(t, func(t *testing.T) {
		listener := &listenerRecorder{}
		h := wirePollerWithListener(t, fx.bc, fx.head, ds, listener)
		h.highest.Store(&core.Header{Number: fx.head.Number + 5}) // not at tip
		go h.poller.Run(t.Context())
		synctest.Wait()

		time.Sleep(tickInterval)
		synctest.Wait()
		require.Empty(t, listener.take())
	})
}
