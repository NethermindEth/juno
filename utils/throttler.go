package utils

import (
	"errors"
	"math"
	"sync/atomic"
)

var ErrResourceBusy = errors.New("resource busy, try again")

type Throttler[T any] struct {
	resource *T
	sem      chan struct{}
	queue    atomic.Int32

	maxQueueLen int32
}

func NewThrottler[T any](concurrencyBudget uint, resource *T) *Throttler[T] {
	return &Throttler[T]{
		resource:    resource,
		sem:         make(chan struct{}, concurrencyBudget),
		maxQueueLen: math.MaxInt32,
	}
}

// WithMaxQueueLen sets the maximum length the queue can grow to
func (t *Throttler[T]) WithMaxQueueLen(maxQueueLen int32) *Throttler[T] {
	t.maxQueueLen = maxQueueLen
	return t
}

// Do lets caller acquire the resource within the context of a callback
func (t *Throttler[T]) Do(doer func(resource *T) error) error {
	queueLen := t.queue.Add(1)
	if queueLen > t.maxQueueLen {
		t.queue.Add(-1)
		return ErrResourceBusy
	}
	t.sem <- struct{}{}
	defer func() {
		<-t.sem
	}()
	t.queue.Add(-1)
	return doer(t.resource)
}

// QueueLen returns the number of Do calls that is blocked on the resource
func (t *Throttler[T]) QueueLen() int {
	return int(t.queue.Load())
}

// JobsRunning returns the number of Do calls that are running at the moment
func (t *Throttler[T]) JobsRunning() int {
	return len(t.sem)
}
