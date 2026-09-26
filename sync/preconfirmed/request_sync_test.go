package preconfirmed_test

import (
	"context"
	"math/rand/v2"
	"runtime"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NethermindEth/juno/sync/preconfirmed"
	"github.com/stretchr/testify/require"
)

const (
	freshnessTime   = 100 * time.Millisecond
	outdatedTime    = freshnessTime + time.Millisecond
	requestWaitTime = 100 * freshnessTime
)

// testWorker runs the worker loop RequestSync is designed for: it serves requests until its
// context is cancelled, holding each work until the test lets it complete with requireWork. A
// successful work increments the value. Tests that must stop the worker at a precise point, such
// as before it picks up a request or in the middle of a work, play the worker from the test
// goroutine instead.
type testWorker struct {
	reqSync           *preconfirmed.RequestSync
	workCh            chan struct{}
	value             int
	shouldUpdateValue bool
}

func newTestWorker(reqSync *preconfirmed.RequestSync) *testWorker {
	return &testWorker{
		reqSync:           reqSync,
		workCh:            make(chan struct{}),
		value:             0,
		shouldUpdateValue: true,
	}
}

// Run executes a task every time there is a request.
func (w *testWorker) Run(ctx context.Context) {
	w.reqSync.Start()
	defer w.reqSync.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case respCh := <-w.reqSync.ListenRequests():
			// Work until the test lets it complete
			w.workCh <- struct{}{}
			if w.shouldUpdateValue {
				w.value += 1
				w.reqSync.SignalSuccess(respCh)
			} else {
				w.reqSync.SignalFailure(respCh)
			}
		}
	}
}

func (w *testWorker) Value() int {
	return w.value
}

func (w *testWorker) ShouldUpdateValue(should bool) {
	w.shouldUpdateValue = should
}

func TestRequestSyncBasic(t *testing.T) {
	// Simple scenario where new values are updated based on if a single request is done inside
	// the freshness windows or not.
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		reqSync, worker := setupTest(ctx)

		// First request should update the value only after the work is done
		go reqSync.Request()
		require.Equal(t, 0, worker.Value())
		requireWork(t, worker)
		require.Equal(t, 1, worker.Value())

		// Second request shouldn't update the value because it is still fresh
		reqDone := request(reqSync)
		requireNoWork(t, worker)
		requireRequestDone(t, reqDone)
		require.Equal(t, 1, worker.Value())

		// Third request shouldn't either because the value is still fresh at the very end of
		// the freshness window
		synctest.Sleep(freshnessTime)
		reqDone = request(reqSync)
		requireNoWork(t, worker)
		requireRequestDone(t, reqDone)

		// Fourth request should update the value because last one is no longer
		// fresh
		synctest.Sleep(time.Nanosecond)
		go reqSync.Request()
		requireWork(t, worker)
		require.Equal(t, 2, worker.Value())

		// Fifth request shouldn't update because even if the request is received in
		// an outdated time, there is nothing to update.
		synctest.Sleep(outdatedTime)
		worker.ShouldUpdateValue(false)
		go reqSync.Request()
		requireWork(t, worker)
		require.Equal(t, 2, worker.Value())

		// Sixth request should update because the previous one didn't update the value
		worker.ShouldUpdateValue(true)
		go reqSync.Request()
		requireWork(t, worker)
		require.Equal(t, 3, worker.Value())
	})
}

func TestRequestSyncWorkDoneBeforeRequestFinishes(t *testing.T) {
	// Scenario to check that requests are never finished **before** the job is done.
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		reqSync, worker := setupTest(ctx)

		// One single request starts and waits for the work to be done before returning
		reqDone := request(reqSync)
		requireRequestNotDone(t, reqDone)
		requireWork(t, worker)
		requireRequestDone(t, reqDone)
		require.Equal(t, 1, worker.Value())

		// Multiple requests wait for the work to be done before returning
		synctest.Sleep(outdatedTime)
		reqDones := requests(reqSync, 10)
		requireRequestNotDone(t, reqDones...)
		requireWork(t, worker)
		requireRequestDone(t, reqDones...)
		require.Equal(t, 2, worker.Value())

		// Multiple requests wait for a failed work as well
		synctest.Sleep(outdatedTime)
		worker.ShouldUpdateValue(false)
		reqDones = requests(reqSync, 10)
		requireRequestNotDone(t, reqDones...)
		requireWork(t, worker)
		requireRequestDone(t, reqDones...)
		require.Equal(t, 2, worker.Value())
	})
}

func TestRequestSyncConcurrent(t *testing.T) {
	// Concurrent requests share a single work, even one that outlasts the freshness window, and
	// the requests right after it reuse its value.
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		reqSync, worker := setupTest(ctx)

		// Check that concurrent go-routines only update the value once
		for range 10 {
			go reqSync.Request()
		}
		require.Equal(t, 0, worker.Value())
		requireWork(t, worker)
		require.Equal(t, 1, worker.Value())

		// Check that follow up go-routines in the same freshness window don't
		// update the value
		reqDones := requests(reqSync, 10)
		requireNoWork(t, worker)
		requireRequestDone(t, reqDones...)
		require.Equal(t, 1, worker.Value())

		// Verify that a work that takes longer than the fresh window doesn't do extra
		// work for requests after the fresh window but while the work wasn't complete yet.
		synctest.Sleep(freshnessTime)
		for range 11 {
			synctest.Sleep(freshnessTime / 10)
			go reqSync.Request()
		}
		requireWork(t, worker)
		require.Equal(t, 2, worker.Value())
		requireNoWork(t, worker)

		// Check that this request still sits inside the fresh data window.
		reqDone := request(reqSync)
		requireNoWork(t, worker)
		requireRequestDone(t, reqDone)

		// Verify that the first five share a single new update, and the next five reuse
		// its value
		synctest.Sleep(freshnessTime)
		for range 5 {
			synctest.Sleep(freshnessTime / 10)
			go reqSync.Request()
		}
		requireWork(t, worker)
		require.Equal(t, 3, worker.Value())
		reqDones = make([]<-chan struct{}, 5)
		for i := range reqDones {
			synctest.Sleep(freshnessTime / 10)
			reqDones[i] = request(reqSync)
		}
		requireNoWork(t, worker)
		requireRequestDone(t, reqDones...)
	})
}

func TestRequestSyncUnexpectedCancel(t *testing.T) {
	// Make sure that requester goroutines never block when the working goroutine cancels
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		reqSync, worker := setupTest(ctx)

		// This test includes cancelling the worker and restarting its jobs, hence this helper.
		restartWorker := func(worker *testWorker) context.CancelFunc {
			ctx, cancel := context.WithCancel(t.Context())
			go worker.Run(ctx)
			synctest.Wait()
			return cancel
		}

		go reqSync.Request()
		requireWork(t, worker)
		require.Equal(t, 1, worker.Value())

		// Make sure that after cancellation requests are not blocked.
		cancel()
		synctest.Wait()

		reqDones := requests(reqSync, 10)
		requireNoWork(t, worker)
		requireRequestDone(t, reqDones...)

		// Check that after restart, the previous fresh window is still considered valid
		cancel = restartWorker(worker)
		reqDone := request(reqSync)
		requireNoWork(t, worker)
		requireRequestDone(t, reqDone)
		require.Equal(t, 1, worker.Value())

		// Cancel the worker while requests wait for its work and more keep arriving. The worker
		// still completes the work, which releases the waiting requests, and then stops. Every
		// request returns, whether it waited for the work, reused its value or found the worker
		// stopped, and no extra work is done.
		synctest.Sleep(outdatedTime)
		reqDones = make([]<-chan struct{}, 100)
		for i := range reqDones {
			reqDones[i] = request(reqSync)
			if i == 50 {
				synctest.Wait()
				cancel()
				// Complete the work without requireWork, which would also wait for the worker
				// to stop, so that the remaining requests arrive while it stops
				<-worker.workCh
			}
		}
		requireRequestDone(t, reqDones...)
		require.Equal(t, 2, worker.Value())
	})
}

func TestRequestSyncSelfRequest(t *testing.T) {
	t.Run("requests join an ongoing self request", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)
			reqSync.Start()
			defer reqSync.Stop()

			// A self request doesn't trigger the requests queue
			respCh := reqSync.SelfRequest()
			requireNoQueuedRequest(t, reqSync)

			// Follow up requests don't trigger the request queue, but they will still wait until
			// the work is done
			reqDones := requests(reqSync, 3)
			requireNoQueuedRequest(t, reqSync)
			requireRequestNotDone(t, reqDones...)

			// Once the work is done, all the requests done in the meantime will also end
			reqSync.SignalSuccess(respCh)
			requireNoQueuedRequest(t, reqSync)
			requireRequestDone(t, reqDones...)
		})
	})

	t.Run("self request takes over a queued request", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)
			reqSync.Start()
			defer reqSync.Stop()

			// A self request drains the queued work
			reqDone := request(reqSync)
			requireRequestNotDone(t, reqDone)
			respCh := reqSync.SelfRequest()
			requireNoQueuedRequest(t, reqSync)
			requireRequestNotDone(t, reqDone)
			reqSync.SignalSuccess(respCh)
			requireNoQueuedRequest(t, reqSync)
			requireRequestDone(t, reqDone)
		})
	})

	t.Run("self request starts work even when data is fresh", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)
			reqSync.Start()
			defer reqSync.Stop()

			// A successful self requested work makes the data fresh
			reqSync.SignalSuccess(reqSync.SelfRequest())
			requireRequestDone(t, request(reqSync))
			requireNoQueuedRequest(t, reqSync)

			// A self request inside the freshness window still starts work. Requests keep reusing the
			// fresh data meanwhile, and once it's outdated they wait for that work instead of asking
			// for new work
			respCh := reqSync.SelfRequest()
			requireRequestDone(t, request(reqSync))
			synctest.Sleep(outdatedTime)
			reqDone := request(reqSync)
			requireRequestNotDone(t, reqDone)
			requireNoQueuedRequest(t, reqSync)

			reqSync.SignalSuccess(respCh)
			requireRequestDone(t, reqDone)
			requireRequestDone(t, request(reqSync))
			requireNoQueuedRequest(t, reqSync)
		})
	})
}

func TestRequestSyncNotRunning(t *testing.T) {
	// Requests never wait for a worker that isn't running, even when the data is outdated. The
	// test goroutine plays the worker.
	synctest.Test(t, func(t *testing.T) {
		reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)

		// Requests before the first start aren't kept for the worker to pick up once it starts
		requireRequestDone(t, request(reqSync))
		reqSync.Start()
		requireNoQueuedRequest(t, reqSync)

		// Same after stopping, which can be done more than once
		reqSync.Stop()
		requireRequestDone(t, request(reqSync))
		reqSync.Stop()
		requireRequestDone(t, request(reqSync))
		requireNoQueuedRequest(t, reqSync)
	})
}

func TestRequestSyncStop(t *testing.T) {
	t.Run("releases a queued request", func(t *testing.T) {
		// The worker can stop before picking up a queued request, e.g. when it is cancelled while
		// busy with something else. Stop must release the requester and drop the request, so a
		// restarted worker isn't handed a request that was already released. The test goroutine
		// plays the worker to control when it stops.
		synctest.Test(t, func(t *testing.T) {
			reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)
			reqSync.Start()

			reqDone := request(reqSync)
			requireRequestNotDone(t, reqDone)
			reqSync.Stop()
			requireRequestDone(t, reqDone)

			reqSync.Start()
			requireNoQueuedRequest(t, reqSync)

			// Following requests are served as usual
			reqDone = request(reqSync)
			respCh := requireQueuedRequest(t, reqSync)
			requireRequestNotDone(t, reqDone)
			reqSync.SignalSuccess(respCh)
			requireRequestDone(t, reqDone)
		})
	})

	t.Run("releases abandoned work", func(t *testing.T) {
		// The worker can stop in the middle of a work without signalling its outcome, e.g. when the
		// work is aborted by a shutdown. Stop must release every request waiting for it, and the
		// abandoned work doesn't count as a success. The test goroutine plays the worker to control
		// when it stops.
		synctest.Test(t, func(t *testing.T) {
			reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)
			reqSync.Start()

			// The worker picks up the first request and never signals it, the others wait for it
			reqDones := requests(reqSync, 1)
			requireQueuedRequest(t, reqSync)
			reqDones = append(reqDones, requests(reqSync, 5)...)
			requireNoQueuedRequest(t, reqSync)
			requireRequestNotDone(t, reqDones...)

			reqSync.Stop()
			requireRequestDone(t, reqDones...)

			// The data is still outdated after restarting, so the next request asks for new work
			reqSync.Start()
			reqDone := request(reqSync)
			reqSync.SignalSuccess(requireQueuedRequest(t, reqSync))
			requireRequestDone(t, reqDone)
		})
	})
}

func TestRequestSyncRequestsStopWaitingForLongWork(t *testing.T) {
	// Requests stop waiting for a work once requestWaitTime has passed since they arrived, whether
	// the worker has picked up the request by then or not.
	synctest.Test(t, func(t *testing.T) {
		reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)
		reqSync.Start()
		defer reqSync.Stop()

		// The first request asks for work and the second one joins it halfway through the first
		// one's wait. Both keep waiting until their own requestWaitTime is over
		first := request(reqSync)
		synctest.Sleep(requestWaitTime / 2)
		second := request(reqSync)
		synctest.Sleep(requestWaitTime/2 - time.Nanosecond)
		requireRequestNotDone(t, first, second)

		// The first request stops waiting before the worker picks up its request, which is still
		// left for the worker
		synctest.Sleep(time.Nanosecond)
		requireRequestDone(t, first)
		requireRequestNotDone(t, second)
		respCh := requireQueuedRequest(t, reqSync)

		// The second request stops waiting in the middle of the work, and a new request joins the
		// work instead of asking for new work
		synctest.Sleep(requestWaitTime / 2)
		requireRequestDone(t, second)
		reqDone := request(reqSync)
		requireNoQueuedRequest(t, reqSync)
		requireRequestNotDone(t, reqDone)

		// Completing the work releases the request still waiting and makes the data fresh
		reqSync.SignalSuccess(respCh)
		requireRequestDone(t, reqDone)
		requireRequestDone(t, request(reqSync))
		requireNoQueuedRequest(t, reqSync)
	})
}

func TestRequestSyncNoRedundantWork(t *testing.T) {
	// Stress tests of requests arriving while a work completes must either wait for it or reuse
	// its value, but never ask for new work.
	// Requesters spin, instead of blocking, until they are released so that they run while the
	// work completes.
	synctest.Test(t, func(t *testing.T) {
		const (
			rounds     = 5000
			requesters = 16
			maxSpins   = 64
		)
		reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)
		reqSync.Start()
		defer reqSync.Stop()

		var spin atomic.Int64
		redundantWorks := 0
		for range rounds {
			synctest.Sleep(outdatedTime)
			go reqSync.Request()
			respCh := requireQueuedRequest(t, reqSync)

			var ready atomic.Int32
			var released atomic.Bool
			for range requesters {
				go func() {
					ready.Add(1)
					for !released.Load() {
						runtime.Gosched()
					}
					reqSync.Request()
				}()
			}
			for ready.Load() < requesters {
				runtime.Gosched()
			}

			// Busy wait a random time before completing the work. Yielding instead would let the
			// requesters run in turns with this goroutine rather than alongside it.
			released.Store(true)
			for range rand.IntN(maxSpins) {
				spin.Add(1)
			}
			reqSync.SignalSuccess(respCh)

			synctest.Wait()
			select {
			case respCh := <-reqSync.ListenRequests():
				redundantWorks++
				reqSync.SignalFailure(respCh)
			default:
			}
		}
		require.Zero(t, redundantWorks, "work requested right after a successful one")
	})
}

func TestRequestSyncRestartsUnderLoad(t *testing.T) {
	// Stress test of the worker stopping and starting again many times while requests keep
	// arriving, including while it is stopped. The worker stops right as a request is asked for,
	// with one waiting to be picked up, in the middle of a work it abandons, or after completing
	// or failing one. It must only be handed requests that are still waited on and asked for
	// outdated data, no request must be left for it while it is stopped, and every request must
	// return once it stops.
	// The test goroutine plays the worker, and requesters spin between requests so that they run
	// alongside it.
	synctest.Test(t, func(t *testing.T) {
		const (
			rounds     = 1000
			restarts   = 8
			requesters = 32
			maxSpins   = 64
		)
		reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)

		var spin atomic.Int64
		busyWait := func() {
			for range rand.IntN(maxSpins) {
				spin.Add(1)
			}
		}

		// Requesters keep asking while requesting is set. A failure in the middle of a round must
		// release them too for the test to end.
		var requesting atomic.Bool
		var running atomic.Int32
		defer func() {
			requesting.Store(false)
			reqSync.Stop()
		}()

		var lastSuccess time.Time
		for range rounds {
			// Time only advances between rounds, while no requester runs, so a success keeps the
			// data fresh until the end of the round
			synctest.Sleep(outdatedTime)
			requesting.Store(true)
			for range requesters {
				running.Add(1)
				go func() {
					defer running.Add(-1)
					for requesting.Load() {
						reqSync.Request()
						runtime.Gosched()
					}
				}()
			}

			for range restarts {
				reqSync.Start()
				// Give the requesters time to ask for work, or stop right as they do
				if rand.IntN(2) == 0 {
					runtime.Gosched()
				}
				// Take the request asked for, if any, unless stopping before picking it up. Then
				// complete it, fail it or stop in the middle of it, abandoning it.
				if rand.IntN(4) > 0 {
					select {
					case respCh := <-reqSync.ListenRequests():
						select {
						case <-respCh:
							t.Fatal("worker handed a request that was already released")
						default:
						}
						require.Greater(
							t,
							time.Since(lastSuccess),
							freshnessTime,
							"work requested while the data was fresh",
						)
						busyWait()
						switch rand.IntN(3) {
						case 0:
							lastSuccess = time.Now()
							reqSync.SignalSuccess(respCh)
						case 1:
							reqSync.SignalFailure(respCh)
						}
					default:
					}
				}
				busyWait()
				reqSync.Stop()

				// Requests keep arriving while the worker is stopped, and none must be left for it
				runtime.Gosched()
				select {
				case <-reqSync.ListenRequests():
					t.Fatal("request left for the stopped worker")
				default:
				}
			}

			requesting.Store(false)
			requireNoQueuedRequest(t, reqSync)
			require.Zero(t, running.Load(), "requests still waiting after the worker stopped")
		}
	})
}

// setupTest creates the two main actors for testing:
//
//	[preconfirmed.RequestSync] the type being tested
//	[testWorker] which runs the main loop to throw request at, until ctx is cancelled
func setupTest(ctx context.Context) (*preconfirmed.RequestSync, *testWorker) {
	reqSync := preconfirmed.NewRequestSync(freshnessTime, requestWaitTime)
	worker := newTestWorker(reqSync)
	go worker.Run(ctx)
	synctest.Wait()

	return reqSync, worker
}

// requireWork requires the worker to be in the middle of a work, then lets the work complete and
// waits for its outcome to be signalled.
func requireWork(t *testing.T, worker *testWorker) {
	t.Helper()

	synctest.Wait()
	select {
	case <-worker.workCh:
	default:
		t.Fatal("expected the worker to be working but it isn't")
	}
	synctest.Wait()
}

// requireNoWork requires the worker not to be working.
func requireNoWork(t *testing.T, worker *testWorker) {
	t.Helper()

	synctest.Wait()
	select {
	case <-worker.workCh:
		t.Fatal("found the worker working when it wasn't expected to")
	default:
	}
}

// request calls [preconfirmed.RequestSync.Request] in a new goroutine. The returned channel is
// closed once it returns. Tests use it instead of `go reqSync.Request()` when they check when
// the request returns.
func request(reqSync *preconfirmed.RequestSync) <-chan struct{} {
	reqDone := make(chan struct{})
	go func() {
		reqSync.Request()
		close(reqDone)
	}()
	return reqDone
}

// requests makes n concurrent requests with [request].
func requests(reqSync *preconfirmed.RequestSync, n int) []<-chan struct{} {
	reqDones := make([]<-chan struct{}, n)
	for i := range reqDones {
		reqDones[i] = request(reqSync)
	}
	return reqDones
}

// requireRequestNotDone requires every request to still be waiting.
func requireRequestNotDone(t *testing.T, reqDones ...<-chan struct{}) {
	t.Helper()

	synctest.Wait()
	for _, reqDone := range reqDones {
		select {
		case <-reqDone:
			t.Fatal("request finished but it was not expected")
		default:
		}
	}
}

// requireRequestDone requires every request to have returned.
func requireRequestDone(t *testing.T, reqDones ...<-chan struct{}) {
	t.Helper()

	synctest.Wait()
	for _, reqDone := range reqDones {
		select {
		case <-reqDone:
		default:
			t.Fatal("request was expected to finish but it hasn't")
		}
	}
}

// requireQueuedRequest returns the request waiting for the worker to pick it up.
func requireQueuedRequest(t *testing.T, reqSync *preconfirmed.RequestSync) chan struct{} {
	t.Helper()

	synctest.Wait()
	select {
	case respCh := <-reqSync.ListenRequests():
		return respCh
	default:
		t.Fatal("expected a queued request but there is none")
		return nil
	}
}

// requireNoQueuedRequest requires no request to be waiting for the worker to pick it up.
func requireNoQueuedRequest(t *testing.T, reqSync *preconfirmed.RequestSync) {
	t.Helper()

	synctest.Wait()
	select {
	case <-reqSync.ListenRequests():
		t.Fatal("found a queued request when none was expected")
	default:
	}
}
