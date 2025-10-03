package tracker_test

import (
	"context"
	"sync/atomic"
	"time"
)

type service struct {
	tracker        waitGroup
	isShutdown     *atomic.Bool
	idGen          *atomic.Uint32
	passedCtxCheck chan struct{}
}

func NewService(tracker waitGroup, isShutdown *atomic.Bool) service {
	service := service{
		tracker:        tracker,
		isShutdown:     isShutdown,
		idGen:          &atomic.Uint32{},
		passedCtxCheck: make(chan struct{}, requests),
	}
	return service
}

func (s *service) handle(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	default:
	}

	// Communicate with the test that ctx.Done() check has passed
	s.passedCtxCheck <- struct{}{}

	// Simulate a delay for some requests to create race condition
	// when acquisition is done after ctx.Done() is closed
	if s.idGen.Add(1) > nonDelayedRequests {
		time.Sleep(acquireDelay)
	}

	isShutdown := s.isShutdown.Load()

	if !s.tracker.Add() {
		return false
	}
	defer s.tracker.Done()

	// Because isShutdown is loaded before, if isShutdown is true while acquiring succeeds,
	// this implies that the acquisition is done after ctx.Done() is closed
	if isShutdown {
		panic("acquired after shutdown")
	}

	// Simulate a delay for some requests to create race condition
	time.Sleep(runtimeDelay)
	return true
}

func (s *service) run(ctx context.Context) {
	<-ctx.Done()
	s.tracker.Wait()
}
