package preconfirmed

import (
	"sync"
	"sync/atomic"
	"time"
)

// Dedicated type to handle multiple goroutines requesting from a single working go routine. Usage:
//
// Main worker Go routine should make sure to call [RequestSync.Start] before accepting incoming
// requests (via [RequestSync.ListenRequests]). [RequestSync.Stop] should be explicitly called
// when work is done, so pending and follow up requests return right away instead of waiting
// out requestWaitTime.
// Finally, during work, (un)successful completion must be signaled accordingly with
// [RequestSync.SignalSuccess] and [RequestSync.SignalFailure].
// If the worker wants to start work without any incoming requests, they can call
// [RequestSync.SelfRequest] and proceed to use [RequestSync.SignalSuccess] or
// [RequestSync.SignalFailure] as if a normal request was sent.
//
// Requester Go routines should limit themselves to only [RequestSync.Request] method which
// will block until the request has been completed (independent of success or failure) or
// requestWaitTime has passed.
type RequestSync struct {
	available             atomic.Bool
	lastSuccessfulRequest atomic.Value
	dataFreshnessTime     time.Duration
	requestWaitTime       time.Duration

	requestCh  chan chan struct{}
	responseCh chan struct{}
	mu         sync.Mutex
}

// NewRequestSync creates a new [RequestSync] that is meant to be shared by many requesting Go
// routines and one main working go routine. You can set `dataFreshnessTime` which is the amount
// of time that will pass from the last successful request before triggering work again, and
// `requestWaitTime` which is the maximum amount of time a request waits for that work to be done.
// It is safe for concurrent use.
// See [RequestSync] type definition for more information into its usage.
func NewRequestSync(dataFreshnessTime, requestWaitTime time.Duration) *RequestSync {
	// an open / closed respCh internally signals an ongoing request. Because
	// no ongoing requests are happening during construction, it starts closed.
	respCh := make(chan struct{})
	close(respCh)

	rs := &RequestSync{
		available:             atomic.Bool{},
		lastSuccessfulRequest: atomic.Value{},
		dataFreshnessTime:     dataFreshnessTime,
		requestWaitTime:       requestWaitTime,

		requestCh:  make(chan chan struct{}, 1),
		responseCh: respCh,
		mu:         sync.Mutex{},
	}
	// stored after construction because an atomic.Value must not be copied after first use
	rs.lastSuccessfulRequest.Store(time.Time{})

	return rs
}

func (rs *RequestSync) Start() {
	rs.available.Store(true)
}

func (rs *RequestSync) Stop() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// stop future requests
	rs.available.Store(false)

	// Close any ongoing request for which work hasn't started yet. Clean the requestCh, for it
	// be empty when Start is called again, and release any of the waiting requesting Go routines.
	select {
	case respCh := <-rs.requestCh:
		close(respCh)
		return
	default:
	}

	// Finally, if `Stop()` was called while handling a request and before signaling success/failure,
	// requesting Go routines whose requestWaitTime hasn't passed yet are still waiting for the
	// signal. Make sure to release those as well.
	if isChannelOpen(rs.responseCh) {
		close(rs.responseCh)
	}
}

// SelfRequest is similar to [RequestSync.Request] and is expected for the working Go routine to
// call it if it's going to do work on its own that would be triggered by a call to
// [RequestSync.Request]. In a similar fashion as to receiving a request. The Go Routine is
// expected to signal success or failure once the self requested work is satisfied.
// If a request is already waiting to be picked up, the self requested work takes it over.
// It must not be called while the worker still has a request or self request to signal, since
// [RequestSync.Stop] would no longer release the requests waiting on that one.
func (rs *RequestSync) SelfRequest() chan struct{} {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// If a Request arrives at the same time as a SelfRequest, then drain the request channel
	// and use that instead to satisfy those pending requests.
	select {
	case respCh := <-rs.requestCh:
		return respCh
	default:
	}

	// Otherwise, create a new one that future concurrent requests can join.
	respCh := make(chan struct{})
	rs.responseCh = respCh
	return respCh
}

// ListenRequests returns a channel to whom notify requests to
func (rs *RequestSync) ListenRequests() <-chan chan struct{} {
	return rs.requestCh
}

// SignalSuccess notifies to a waiting Go routine that its request is done
// and updates the last successful time.
func (rs *RequestSync) SignalSuccess(respCh chan struct{}) {
	// The time must be updated before closing respCh: [RequestSync.Request] relies on it to not
	// ask for new work right after a successful one.
	rs.lastSuccessfulRequest.Store(time.Now())
	rs.signal(respCh)
}

// SignalFailure notifies to a waiting go routine that its request is done.
func (rs *RequestSync) SignalFailure(respCh chan struct{}) {
	rs.signal(respCh)
}

// Request asks the worker to do its work and waits for it to be done, successfully or not, for
// at most requestWaitTime. Once that time has passed it returns while the work carries on. If
// there is an ongoing request already, it joins it. It returns right away if
// the RequestSync is not running or the data is still fresh, and early if the RequestSync stops.
func (rs *RequestSync) Request() {
	// Stop case added here for Requesting Go Routines that don't need to wait to exit cleanly and
	// not contribute nor face contention. Micro-benchmarks shows that these `Request` time is
	// reduced from 1.5 micro seconds to a few nano seconds.
	if !rs.available.Load() || rs.isDataFresh() {
		return
	}

	rs.mu.Lock()
	// Check again here, in case there was Stop() call ran in between.
	if !rs.available.Load() {
		rs.mu.Unlock()
		return
	}

	respCh := rs.responseCh
	if !isChannelOpen(respCh) {
		// Check recency again, now that the latest request is seen as done. SignalSuccess
		// first updates the recency and then closes the channel, allowing us to deterministically
		// check if the channel was closed due to a concurrent call to SingalSuccess.
		if rs.isDataFresh() {
			rs.mu.Unlock()
			return
		}

		respCh = make(chan struct{})
		rs.responseCh = respCh
		rs.requestCh <- respCh
	}
	rs.mu.Unlock()

	// Requests are free to drop out of waiting without interrupting the work
	select {
	case <-respCh:
	case <-time.After(rs.requestWaitTime):
	}
}

func (rs *RequestSync) signal(respCh chan struct{}) {
	close(respCh)
}

func (rs *RequestSync) isDataFresh() bool {
	lastSuccessfulRequest := rs.lastSuccessfulRequest.Load().(time.Time)
	return time.Since(lastSuccessfulRequest) <= rs.dataFreshnessTime
}

func isChannelOpen(ch chan struct{}) bool {
	open := true
	select {
	case _, open = <-ch:
	default:
	}

	return open
}
