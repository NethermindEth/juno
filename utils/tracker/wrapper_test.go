package tracker_test

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

const semaphoreThreshold = 1000000

type waitGroupWrapper struct {
	sync.WaitGroup
}

func (w *waitGroupWrapper) Add(delta int) bool {
	w.WaitGroup.Add(delta)
	return true
}

type semaphoreWrapper struct {
	*semaphore.Weighted
	ctx context.Context
}

func NewSemaphoreWrapper(ctx context.Context) *semaphoreWrapper {
	return &semaphoreWrapper{
		semaphore.NewWeighted(semaphoreThreshold),
		ctx,
	}
}

func (s *semaphoreWrapper) Add(delta int) bool {
	return s.Weighted.Acquire(s.ctx, int64(delta)) == nil
}

func (s *semaphoreWrapper) Done() {
	s.Weighted.Release(1)
}

func (s *semaphoreWrapper) Wait() {
	_ = s.Weighted.Acquire(s.ctx, semaphoreThreshold)
}
