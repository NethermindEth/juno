package utils

import (
	"context"
	"errors"
	"sync/atomic"
)

var ErrResourceBusy = errors.New("resource busy, try again")

type Throttler[T any] struct {
	resource    *T
	runningJobs atomic.Int64
	queue       chan unitOfWork[T]
}

type unitOfWork[T any] struct {
	doer func(resource *T) error
	err  chan error
}

func NewThrottler[T any](queueLen int32, concurrencyBudget uint, resource *T) *Throttler[T] {
	queue := make(chan unitOfWork[T], queueLen)
	throttler := &Throttler[T]{
		resource: resource,
		queue:    queue,
	}

	for range concurrencyBudget {
		go func() {
			for unit := range queue {
				throttler.runningJobs.Add(1)
				unit.err <- unit.doer(resource)
				throttler.runningJobs.Add(-1)
			}
		}()
	}

	return throttler
}

// Do lets caller acquire the resource within the context of a callback
func (t *Throttler[T]) Do(ctx context.Context, doer func(resource *T) error) error {
	workUnit := t.newUnitOfWork(doer)

	select {
	case t.queue <- workUnit:
		return <-workUnit.err
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrResourceBusy
	}
}

// QueueLen returns the number of Do calls that is blocked on the resource
func (t *Throttler[T]) QueueLen() int {
	return len(t.queue)
}

// JobsRunning returns the number of Do calls that are running at the moment
func (t *Throttler[T]) JobsRunning() int {
	return int(t.runningJobs.Load())
}

func (t *Throttler[T]) newUnitOfWork(doer func(*T) error) unitOfWork[T] {
	// todo use sync.Pool or pointer?
	return unitOfWork[T]{
		err:  make(chan error),
		doer: doer,
	}
}
