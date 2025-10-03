package tracker_test

import (
	"context"
	"sync/atomic"
	"time"
)

type barrier struct {
	checkCh chan struct{}
	doneCh  chan struct{}
}

func newBarrier(size int) barrier {
	checkCh := make(chan struct{}, size)
	doneCh := make(chan struct{})

	go func() {
		for range size {
			<-checkCh
		}
		close(doneCh)
	}()

	return barrier{
		checkCh: checkCh,
		doneCh:  doneCh,
	}
}

func (b barrier) check() {
	b.checkCh <- struct{}{}
}

func (b barrier) done() {
	<-b.doneCh
}

type service struct {
	tracker            waitGroup
	isShutdown         *atomic.Bool
	passedCtxCheck     barrier
	finishedNonDelayed barrier
}

func NewService(tracker waitGroup, isShutdown *atomic.Bool, nonDelayed, delayed int) service {
	service := service{
		tracker:            tracker,
		isShutdown:         isShutdown,
		passedCtxCheck:     newBarrier(nonDelayed + delayed),
		finishedNonDelayed: newBarrier(nonDelayed),
	}
	return service
}

func (s *service) handle(ctx context.Context, isDelayed bool) bool {
	select {
	case <-ctx.Done():
		return false
	default:
	}

	// Communicate with the test that ctx.Done() check has passed
	s.passedCtxCheck.check()

	// We track the completion of the non delayed requests, then wait until they are all completed
	// so that the Wait() at call site completes. Then we can check if any acquisition is done
	// after Wait() completes.
	if isDelayed {
		s.finishedNonDelayed.done()
	} else {
		defer s.finishedNonDelayed.check()
	}

	isShutdown := s.isShutdown.Load()

	if !s.tracker.Add(1) {
		return false
	}
	defer s.tracker.Done()

	// Because isShutdown is loaded before, if isShutdown is true while acquiring succeeds,
	// this implies that the acquisition is done after ctx.Done() is closed
	if isShutdown {
		panic("acquired after shutdown")
	}

	// Simulate a small delay to create async boundary to increase the chances of race condition
	time.Sleep(asyncBoundaryDelay)
	return true
}

func (s *service) run(ctx context.Context) {
	<-ctx.Done()
	s.tracker.Wait()
}
