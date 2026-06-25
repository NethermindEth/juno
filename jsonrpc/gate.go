package jsonrpc

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
)

// note(rdr): Gate is a specific implementation of the [throttler.Throttler] type.
// It is intentionally done this way because it sits at the hot path of the json-rpc
// and needs to be as performant as possible.

// ErrServerBusy is returned by [Gate.Acquire] when the gate is at capacity
// (all concurrent slots are busy and the queue is full).
var ErrServerBusy = errors.New("too many requests")

// Gate is an admission controller for incoming requests. It admits up to
// maxConcurrent requests for immediate processing, queues up to maxQueue more
// and rejects anything beyond.
type Gate struct {
	sem chan struct{} // capacity == maxConcurrent

	// activeRequests counts requests currently processing or queued.
	activeRequests atomic.Uint64
	// rejected counts requests turned away with ErrServerBusy (monotonic).
	rejected atomic.Uint64
	// maxRequests is the total admitted at once (maxConcurrent + maxQueue).
	maxRequests uint64
}

// NewGate returns a Gate that processes up to maxConcurrent requests at once and
// queues up to maxQueue more before rejecting.
func NewGate(maxConcurrent uint, maxQueue uint64) *Gate {
	// Guard against overflow, mirroring throttler.NewThrottler.
	maxRequests := maxQueue + uint64(maxConcurrent)
	if maxRequests < maxQueue {
		maxRequests = math.MaxUint64
	}

	return &Gate{
		sem:         make(chan struct{}, maxConcurrent),
		maxRequests: maxRequests,
	}
}

// Acquire reserves a processing slot. Queues (blocks) all concurrent slots
// are busy. It returns ErrServerBusy if the queue is full or ctx.Err() if context
// is cancelled while waiting.
func (g *Gate) Acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err // already cancelled, don't even enter the queue
	}

	if g.increaseActiveReq() > g.maxRequests {
		g.decreaseActivReq()
		g.rejected.Add(1)
		return ErrServerBusy
	}

	select {
	case g.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		g.decreaseActivReq()
		return ctx.Err()
	}
}

// Release frees a processing slot previously taken by a successful Acquire.
func (g *Gate) Release() {
	<-g.sem
	g.decreaseActivReq()
}

// Running returns the number of requests currently being processed.
func (g *Gate) Running() int { return len(g.sem) }

// Queued returns the number of requests waiting for a processing slot.
func (g *Gate) Queued() int { return int(g.activeRequests.Load()) - len(g.sem) }

// Rejected returns the total number of requests turned away with ErrServerBusy.
func (g *Gate) Rejected() uint64 { return g.rejected.Load() }

func (g *Gate) increaseActiveReq() uint64 {
	return g.activeRequests.Add(1)
}

func (g *Gate) decreaseActivReq() {
	g.activeRequests.Add(^uint64(0))
}
