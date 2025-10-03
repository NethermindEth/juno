package tracker_test

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

type waitGroupWrapper struct {
	sync.WaitGroup
}

func (w *waitGroupWrapper) Add() bool {
	w.WaitGroup.Add(1)
	return true
}

type semaphoreWrapper struct {
	*semaphore.Weighted
	ctx context.Context
}

func NewSemaphoreWrapper(ctx context.Context) *semaphoreWrapper {
	return &semaphoreWrapper{
		semaphore.NewWeighted(requests),
		ctx,
	}
}

func (s *semaphoreWrapper) Add() bool {
	return s.Weighted.Acquire(s.ctx, 1) == nil
}

func (s *semaphoreWrapper) Done() {
	s.Weighted.Release(1)
}

func (s *semaphoreWrapper) Wait() {
	s.Weighted.Acquire(s.ctx, requests)
}
