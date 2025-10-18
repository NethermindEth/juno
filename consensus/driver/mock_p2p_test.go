package driver_test

import (
	"context"
	"sync"
)

type mockListener[M any] struct {
	ch chan M
}

func newMockListener[M any](ch chan M) *mockListener[M] {
	return &mockListener[M]{
		ch: ch,
	}
}

func (m *mockListener[M]) Listen() <-chan M {
	return m.ch
}

type mockBroadcaster[M any] struct {
	wg                  sync.WaitGroup
	mu                  sync.Mutex
	broadcastedMessages []M
}

func (m *mockBroadcaster[M]) Broadcast(ctx context.Context, msg M) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastedMessages = append(m.broadcastedMessages, msg)
	m.wg.Done()
}
