package throttler

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
)

var ErrResourceBusy = errors.New("resource busy, try again")

// Throttler limits how many times an action is done concurrently
// and how many requests for these actions can be queued at max.
type Throttler[T any] struct {
	resource *T
	sem      chan struct{}

	// currentRequests counts current active and queued requests
	currentRequests atomic.Uint64
	// maxRequests is the total of possible requests (active + queued)
	maxRequests uint64
}

type options struct {
	maxQueueLen uint64
}

type Option func(*options)

// WithMaxQueueLen sets the maximum length the queue can grow to.
func WithMaxQueueLen(maxQueueLen uint64) Option {
	return func(o *options) {
		o.maxQueueLen = maxQueueLen
	}
}

// NewThrottler returns a new throttler that will allow up to `maxConcurrentReqs` concurrent
// requests for resource `T`. See [throttler.Option] for other options.
func NewThrottler[T any](maxConcurrentReqs uint, resource *T, opts ...Option) *Throttler[T] {
	o := options{
		maxQueueLen: 1024,
	}
	for _, opt := range opts {
		opt(&o)
	}

	// guard against overflow
	maxRequests := o.maxQueueLen + uint64(maxConcurrentReqs)
	if maxRequests < o.maxQueueLen {
		maxRequests = math.MaxUint64
	}

	return &Throttler[T]{
		resource: resource,
		sem:      make(chan struct{}, maxConcurrentReqs),

		currentRequests: atomic.Uint64{},
		maxRequests:     maxRequests,
	}
}

// Do lets caller acquire the resource within the context of a callback
func (t *Throttler[T]) Do(ctx context.Context, doer func(resource *T) error) error {
	if err := ctx.Err(); err != nil {
		return err // already cancelled, don't even enter the queue
	}

	activeReqs := t.currentRequests.Add(1)
	defer t.currentRequests.Add(^uint64(0)) // decrement by 1
	if activeReqs > t.maxRequests {
		return ErrResourceBusy
	}

	select {
	case t.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	defer func() {
		<-t.sem
	}()
	return doer(t.resource)
}

// QueueLen returns the number of Do calls that is blocked on the resource
func (t *Throttler[T]) QueueLen() int {
	return int(t.currentRequests.Load()) - len(t.sem)
}

// JobsRunning returns the number of Do calls that are running at the moment
func (t *Throttler[T]) JobsRunning() int {
	return len(t.sem)
}
