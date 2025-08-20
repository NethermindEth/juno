//go:build !ring

package broadcast

import (
	"math/bits"
	"sync"
)

type item[T any] struct {
	msg       T
	seq       uint64
	isWriting sync.RWMutex
}

func (i *item[T]) read() (T, uint64) {
	i.isWriting.RLock()
	defer i.isWriting.RUnlock()
	return i.msg, i.seq
}

type broadcastChan[T any] struct {
	buffer   []item[T]
	tail     uint64
	capacity uint64
	mask     uint64
	subs     map[uint64]*subscriptionChan[T]
	nextSub  uint64
	mu       sync.Mutex
	done     chan struct{}
}

func New[T any](capacity uint64) Broadcast[T] {
	capacity = 1 << bits.Len64(capacity-1)
	buffer := make([]item[T], capacity)
	buffer[0].isWriting.Lock()
	return &broadcastChan[T]{
		buffer:   buffer,
		tail:     0,
		capacity: capacity,
		mask:     capacity - 1,
		mu:       sync.Mutex{},
		subs:     make(map[uint64]*subscriptionChan[T]),
		nextSub:  0,
		done:     make(chan struct{}),
	}
}

func (b *broadcastChan[T]) Send(msg T) error {
	select {
	case <-b.done:
		return ErrClosed
	default:
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	tail := b.tail & b.mask
	next := (b.tail + 1) & b.mask
	b.buffer[next].isWriting.Lock()
	b.buffer[next].seq = b.tail + 1

	b.buffer[tail].msg = msg
	b.buffer[tail].isWriting.Unlock()
	b.tail++
	return nil
}

func (b *broadcastChan[T]) Subscribe() Subscription[T] {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := subscriptionChan[T]{
		broadcast: b,
		pos:       b.tail,
		out:       make(chan EventOrLag[T], 1),
		done:      make(chan struct{}),
	}

	b.subs[b.nextSub] = &sub
	b.nextSub++

	go sub.run()
	return &sub
}

func (b *broadcastChan[T]) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	close(b.done)
	tail := b.tail & b.mask
	b.buffer[tail].seq = 0
	b.buffer[tail].isWriting.Unlock()

	for _, s := range b.subs {
		s.Unsubscribe()
	}
}

type subscriptionChan[T any] struct {
	broadcast *broadcastChan[T]
	pos       uint64
	out       chan EventOrLag[T]
	done      chan struct{}
}

func (s *subscriptionChan[T]) run() {
	defer close(s.out)

	for {
		idx := s.pos & s.broadcast.mask
		msg, pos := s.broadcast.buffer[idx].read()

		var event EventOrLag[T]
		if pos != s.pos {
			s.broadcast.mu.Lock()
			next := s.broadcast.tail - s.broadcast.capacity + 1
			event = EventOrLag[T]{
				lag: LaggedInfo{
					MissedSeq: s.pos + 1,
					NextSeq:   next + 1,
				},
				isLag: true,
			}
			s.pos = next
			s.broadcast.mu.Unlock()
		} else {
			event = EventOrLag[T]{
				event: msg,
			}
			s.pos++
		}

		select {
		case <-s.done:
			return
		case s.out <- event:
		}
	}
}

func (s *subscriptionChan[T]) Unsubscribe() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
	}
}

func (s *subscriptionChan[T]) Recv() <-chan EventOrLag[T] {
	return s.out
}
