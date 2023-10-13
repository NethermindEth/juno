package feed

import (
	"sync"
)

type Feed[T any] struct {
	mu     sync.Mutex // protects subs and nextID.
	subs   map[uint64]*Subscription[T]
	nextID uint64
}

type Subscription[T any] struct {
	c         chan T
	f         *Feed[T]
	unsubOnce sync.Once
	id        uint64
}

func (s *Subscription[T]) Recv() <-chan T {
	return s.c
}

func (s *Subscription[T]) Unsubscribe() {
	s.unsubOnce.Do(func() {
		s.f.mu.Lock()
		defer s.f.mu.Unlock()
		close(s.c)
		delete(s.f.subs, s.id)
	})
}

func New[T any]() *Feed[T] {
	return &Feed[T]{
		subs: make(map[uint64]*Subscription[T], 0),
	}
}

func (f *Feed[T]) Subscribe() *Subscription[T] {
	ch := make(chan T, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	s := &Subscription[T]{
		c:  ch,
		f:  f,
		id: f.nextID,
	}
	f.nextID++
	f.subs[s.id] = s
	return s
}

// Send broadcasts v to all subscribers. Send will skip subscribers that block.
// Subscribers risk reading stale data if they wait a long time before calling Recv.
func (f *Feed[T]) Send(v T) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, sub := range f.subs {
		select {
		case sub.c <- v:
		default:
		}
	}
}

// Tee forwards all values received from sub to f.
// It stops tee-ing values when sub is unsubscribed.
func Tee[T any](sub *Subscription[T], f *Feed[T]) {
	go func() {
		for v := range sub.Recv() {
			f.Send(v)
		}
	}()
}
