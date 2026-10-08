package preconfirmed_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// longTick keeps periodic polls outside every test's horizon: the tests below drive the poller
// through reads, so a tick would blur which poll served a read.
const longTick = 10 * time.Second

// wirePollerWithInterval is [wirePoller] with a chosen tick interval. It keeps the package's
// staleAfter and onDemandWait so reads behave as in the rest of the tests.
func wirePollerWithInterval(
	t *testing.T,
	bc *blockchain.Blockchain,
	head *core.Header,
	ds preconfirmed.DataSource,
	interval time.Duration,
) harness {
	t.Helper()
	highest := &atomic.Pointer[core.Header]{}
	highest.Store(head)

	p := preconfirmed.NewPoller(
		ds,
		bc,
		highest,
		interval,
		staleAfter,
		onDemandWait,
		&preconfirmed.SelectiveListener{},
		log.NewNopZapLogger(),
	)
	return harness{poller: p, highest: highest}
}

// chainRead is what one PreConfirmedChain call returned and how long it waited for it.
type chainRead struct {
	view    preconfirmed.ChainReader
	err     error
	elapsed time.Duration
}

// requestChain reads the pre-confirmed chain in a new goroutine, the way an RPC handler would,
// and delivers the result once the read returns. In these tests a read is a request: it asks
// for a poll whenever the stored chain is stale and waits for it, so tests collect the result
// with [requireReadReturned] or check on it with [requireReadsWaiting].
func (h harness) requestChain() <-chan chainRead {
	start := time.Now()
	// Buffered so that a result nobody collects never leaves the goroutine parked.
	done := make(chan chainRead, 1)
	go func() {
		view, err := h.poller.PreConfirmedChain()
		done <- chainRead{view: view, err: err, elapsed: time.Since(start)}
	}()
	return done
}

// requireReadsWaiting requires every read to still be waiting for its poll.
func requireReadsWaiting(t *testing.T, reads ...<-chan chainRead) {
	t.Helper()

	synctest.Wait()
	for i, read := range reads {
		select {
		case <-read:
			t.Fatalf("read %d returned but it was expected to keep waiting", i)
		default:
		}
	}
}

// requireReadsReturned requires every read to have returned without error and hands back their
// results in the same order.
func requireReadsReturned(t *testing.T, reads ...<-chan chainRead) []chainRead {
	t.Helper()

	synctest.Wait()
	results := make([]chainRead, 0, len(reads))
	for i, read := range reads {
		select {
		case result := <-read:
			require.NoError(t, result.err, "read %d", i)
			results = append(results, result)
		default:
			t.Fatalf("read %d was expected to return but it is still waiting", i)
		}
	}
	return results
}

// requireReadReturned is [requireReadsReturned] for a single read.
func requireReadReturned(t *testing.T, read <-chan chainRead) chainRead {
	t.Helper()
	return requireReadsReturned(t, read)[0]
}

// A read finding nothing polled yet asks for a poll and answers with its result: no tick is
// needed, and the read waits for the poll it asked for rather than for the on-demand wait.
// Once the chain is stale, concurrent reads are all served by a single poll.
func TestPollerRequestPollsOnDemand(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r0", 1)
	delta := makeTestDelta("r0", 1)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockPreConfirmedDataSource(ctrl)
	gomock.InOrder(
		// The first read, with the chain still empty.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(block1, uint64(1), nil),
		// The concurrent reads once the chain is stale: one poll, continuing from the stored tip.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(1)).
			Return(delta, uint64(1), nil),
	)

	synctest.Test(t, func(t *testing.T) {
		harness := wirePollerWithInterval(t, fx.bc, fx.head, ds, longTick)
		go harness.poller.Run(t.Context())
		synctest.Wait()

		// The read asks for a poll and returns as soon as it is done, with no time passing: it
		// waited neither for a tick nor for the on-demand wait.
		first := requireReadReturned(t, harness.requestChain())
		require.Zero(t, first.elapsed, "the read waited for something other than its poll")
		assertChain(t, &first.view, entry(1, &block1))

		// The chain goes stale and several reads arrive at once: one poll serves them all.
		synctest.Sleep(staleAfter + time.Nanosecond)
		reads := make([]<-chan chainRead, 5)
		for i := range reads {
			reads[i] = harness.requestChain()
		}
		for i, read := range requireReadsReturned(t, reads...) {
			require.Zero(t, read.elapsed, "read %d waited for something other than its poll", i)
			assertChain(t, &read.view, entry(1, &block1, &delta))
		}
	})
}

// Reads inside the freshness window are answered from the stored chain without polling, whether
// the last poll was asked for by a read or ran on a tick. Once the window is over, a read polls.
func TestPollerRequestReusesFreshData(t *testing.T) {
	t.Parallel()

	t.Run("after an on-demand poll", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		block1 := makeTestPreConfirmedBlock("r0", 1)
		delta := makeTestDelta("r0", 1)

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockPreConfirmedDataSource(ctrl)
		gomock.InOrder(
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(block1, uint64(1), nil),
			// Only the read past the window polls: an earlier poll would land the delta too
			// soon and change the chain the reads inside the window see.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(1)).
				Return(delta, uint64(1), nil),
		)

		synctest.Test(t, func(t *testing.T) {
			harness := wirePollerWithInterval(t, fx.bc, fx.head, ds, longTick)
			go harness.poller.Run(t.Context())
			synctest.Wait()

			read := requireReadReturned(t, harness.requestChain())
			assertChain(t, &read.view, entry(1, &block1))

			// Reads right after the poll, halfway through the window and at its very end are
			// all answered from the stored chain right away.
			for _, pause := range []time.Duration{0, staleAfter / 2, staleAfter / 2} {
				synctest.Sleep(pause)
				read = requireReadReturned(t, harness.requestChain())
				require.Zero(t, read.elapsed, "a fresh read waited")
				assertChain(t, &read.view, entry(1, &block1))
			}

			// Right past the window a read polls again and answers with the new data.
			synctest.Sleep(time.Nanosecond)
			read = requireReadReturned(t, harness.requestChain())
			assertChain(t, &read.view, entry(1, &block1, &delta))

			// The successful poll reopened the window.
			read = requireReadReturned(t, harness.requestChain())
			assertChain(t, &read.view, entry(1, &block1, &delta))
		})
	})

	t.Run("after a tick", func(t *testing.T) {
		t.Parallel()
		fx := newChainFixture(t)

		block1 := makeTestPreConfirmedBlock("r0", 1)
		delta := makeTestDelta("r0", 1)
		// Long enough for the window to close between ticks: a tick landing at the window's
		// end would keep it open for good.
		tick := 10 * staleAfter

		ctrl := gomock.NewController(t)
		ds := mocks.NewMockPreConfirmedDataSource(ctrl)
		gomock.InOrder(
			// The first read.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
				Return(block1, uint64(1), nil),
			// The tick.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(1)).
				Return(delta, uint64(1), nil),
			// The read once the tick's window is over.
			ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(2)).
				Return(starknet.PreConfirmedNoChange{}, uint64(1), nil),
		)

		synctest.Test(t, func(t *testing.T) {
			harness := wirePollerWithInterval(t, fx.bc, fx.head, ds, tick)
			go harness.poller.Run(t.Context())
			synctest.Wait()

			read := requireReadReturned(t, harness.requestChain())
			assertChain(t, &read.view, entry(1, &block1))

			// The tick polls on its own and lands the delta.
			synctest.Sleep(tick)

			// A read right after the tick is fresh: the tick's data, no poll and no wait.
			read = requireReadReturned(t, harness.requestChain())
			require.Zero(t, read.elapsed, "a fresh read waited")
			assertChain(t, &read.view, entry(1, &block1, &delta))

			// The tick's window closes like any other: the next read polls.
			synctest.Sleep(staleAfter + time.Nanosecond)
			read = requireReadReturned(t, harness.requestChain())
			assertChain(t, &read.view, entry(1, &block1, &delta))
		})
	})
}

// A failed poll does not count: the read it served answers with what is stored, right away, and
// the next read asks for a poll again.
func TestPollerRequestRepollsAfterFailure(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r0", 1)
	delta := makeTestDelta("r0", 1)

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockPreConfirmedDataSource(ctrl)
	gomock.InOrder(
		// The first read fails to bootstrap the chain; the read right after it tries again.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(nil, uint64(0), errors.New("wire boom")),
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(block1, uint64(1), nil),
		// Once stale, a read fails to update the chain; the read right after it tries again.
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(1)).
			Return(nil, uint64(0), errors.New("wire boom")),
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(1)).
			Return(delta, uint64(1), nil),
	)

	synctest.Test(t, func(t *testing.T) {
		harness := wirePollerWithInterval(t, fx.bc, fx.head, ds, longTick)
		go harness.poller.Run(t.Context())
		synctest.Wait()

		// The failed poll leaves nothing behind and releases the read at once: it answers with
		// the placeholder rather than sitting out the on-demand wait.
		read := requireReadReturned(t, harness.requestChain())
		require.Zero(t, read.elapsed, "the read outlived its failed poll")
		assertChain(t, &read.view, blankEntry(1))

		// The next read is not fooled by the failure: it polls again.
		read = requireReadReturned(t, harness.requestChain())
		assertChain(t, &read.view, entry(1, &block1))

		// The same holds once the chain is stale, with the read keeping what is stored.
		synctest.Sleep(staleAfter + time.Nanosecond)
		read = requireReadReturned(t, harness.requestChain())
		require.Zero(t, read.elapsed, "the read outlived its failed poll")
		assertChain(t, &read.view, entry(1, &block1))

		read = requireReadReturned(t, harness.requestChain())
		assertChain(t, &read.view, entry(1, &block1, &delta))

		// The successful poll reopened the window: no more polls.
		read = requireReadReturned(t, harness.requestChain())
		assertChain(t, &read.view, entry(1, &block1, &delta))
	})
}

// A poll that outlives the on-demand wait does not hold reads hostage: each answers with the
// stale chain at its own deadline while the poll carries on, and the result still serves
// whoever is waiting once it lands.
func TestPollerRequestStopsWaitingForSlowPoll(t *testing.T) {
	t.Parallel()
	fx := newChainFixture(t)

	block1 := makeTestPreConfirmedBlock("r0", 1)
	delta := makeTestDelta("r0", 1)

	// Made inside the bubble, so that the poll blocked on it counts as durably blocked and the
	// bubble's clock can move on to the reads' deadlines.
	var release chan struct{}
	slowPoll := func(
		context.Context, string, uint64,
	) (starknet.PreConfirmedUpdate, uint64, error) {
		<-release
		return delta, uint64(1), nil
	}

	ctrl := gomock.NewController(t)
	ds := mocks.NewMockPreConfirmedDataSource(ctrl)
	gomock.InOrder(
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "", uint64(0)).
			Return(block1, uint64(1), nil),
		ds.EXPECT().PreConfirmedBlockLatest(gomock.Any(), "r0", uint64(1)).
			DoAndReturn(slowPoll),
	)

	synctest.Test(t, func(t *testing.T) {
		release = make(chan struct{})
		// Released however the test ends: a poller left blocked in the poll would turn a plain
		// failure into a bubble deadlock.
		releaseOnce := sync.OnceFunc(func() { close(release) })
		defer releaseOnce()

		harness := wirePollerWithInterval(t, fx.bc, fx.head, ds, longTick)
		go harness.poller.Run(t.Context())
		synctest.Wait()

		bootstrap := requireReadReturned(t, harness.requestChain())
		assertChain(t, &bootstrap.view, entry(1, &block1))
		synctest.Sleep(staleAfter + time.Nanosecond)

		// Two stale reads ask for a poll that does not finish; a third joins it halfway
		// through their wait.
		first, second := harness.requestChain(), harness.requestChain()
		requireReadsWaiting(t, first, second)
		synctest.Sleep(onDemandWait / 2)
		third := harness.requestChain()
		requireReadsWaiting(t, first, second, third)

		// At their deadline the first two reads give up on the poll and answer with the stale
		// chain, while the third keeps waiting for its own deadline.
		synctest.Sleep(onDemandWait / 2)
		for i, read := range requireReadsReturned(t, first, second) {
			require.Equal(t, onDemandWait, read.elapsed, "read %d", i)
			assertChain(t, &read.view, entry(1, &block1))
		}
		requireReadsWaiting(t, third)

		// The poll was never interrupted: once it lands, its data answers the waiting read.
		releaseOnce()
		late := requireReadReturned(t, third)
		require.Equal(t, onDemandWait/2, late.elapsed)
		assertChain(t, &late.view, entry(1, &block1, &delta))

		// And the late result still reopened the window.
		fresh := requireReadReturned(t, harness.requestChain())
		require.Zero(t, fresh.elapsed, "a fresh read waited")
		assertChain(t, &fresh.view, entry(1, &block1, &delta))
	})
}
