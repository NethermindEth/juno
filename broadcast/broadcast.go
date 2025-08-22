// Package broadcast implements fan-out event streaming and notification systems
// where multiple consumers need to receive all messages or be informed about lost messages,
// while the producers manages a bounded buffer with overwrites.
// Safe for multi-producer multi-consumer setting.
package broadcast

import (
	"context"
	"errors"
	"math/bits"
	"sync"
	"sync/atomic"
	"unsafe"
)

// EventOrLag is a tagged union:
//   - If isLag is false, event holds a regular message.
//   - If isLag is true, lag holds the lag notification.
type EventOrLag[T any] struct {
	event T           // For a regular message
	lag   LaggedError // For a lag notification
	isLag bool
}

// IsEvent returns true if the EventOrLag contains a regular Event.
func (e *EventOrLag[T]) IsEvent() bool {
	return !e.isLag
}

// IsLag returns true if the EventOrLag contains a lag notification.
func (e *EventOrLag[T]) IsLag() bool {
	return e.isLag
}

// Event returns the event value or an error if none.
func (e *EventOrLag[T]) Event() (T, error) {
	var zero T
	if e.isLag {
		return zero, errors.New("no event present")
	}
	return e.event, nil
}

// Lag returns the lag info or an error if none.
func (e *EventOrLag[T]) Lag() (LaggedError, error) {
	if !e.isLag {
		return LaggedError{}, errors.New("no lag info present")
	}
	return e.lag, nil
}

// Subscription represents one consumer.
//   - bcast: back-reference to the Broadcast.
//   - nextSeq: next sequence to read.
//   - out: user-facing channel for events or lag notifications.
type Subscription[T any] struct {
	bcast   *Broadcast[T]
	nextSeq uint64

	out chan EventOrLag[T] // user-facing channel for messages
	ctx context.Context
}

// run is the delivery goroutine for a subscription.
//   - Repeatedly tries to receive from ring at sub.nextSeq.
//   - On event: sends to out (blocks unless done is closed).
//   - On lag: updates sub.nextSeq to NextSeq and emits a lag notification.
//   - On close:
//     If broadcast is closed drain-on-close semantics on. Drains the buffer then exits and closes out.
//     If subscription is closed via Unsubscribe, returns immediately upon receiving done signal.
//     run exits by closing the out channel so consumer knows no further messages.
func (sub *Subscription[T]) run() {
	defer close(sub.out)
	bcast := sub.bcast

	for {
		// Drain-on-close semantics. Drains the buffer even if closed.
		for sub.nextSeq >= bcast.tail.Load() {
			// slow path, wait for next
			notify := bcast.loadNotify()
			select {
			case <-notify:
				continue
			case <-bcast.ctx.Done():
				return
			case <-sub.ctx.Done():
				return
			}
		}

		idx := sub.nextSeq & bcast.mask
		msg, slotSeq := bcast.buffer[idx].read()
		var res EventOrLag[T]
		if slotSeq == sub.nextSeq {
			sub.nextSeq++
			res.event = msg
			res.isLag = false
		} else {
			// Check if queried seq. is overwritten.
			// We should end up this branch only on overwrite thus newest -rb.capacity should never underflow.
			newest := bcast.tail.Load() - 1
			oldest := newest - bcast.capacity + 1

			res.lag.MissedSeq = sub.nextSeq
			res.lag.NextSeq = oldest
			res.isLag = true

			sub.nextSeq = oldest
		}

		select {
		case sub.out <- res:
			continue
		case <-sub.ctx.Done():
			return
		}
	}
}

// Recv returns the user-facing channel for this subscription.
func (sub *Subscription[T]) Recv() <-chan EventOrLag[T] {
	return sub.out
}

// A single ring buffer slot with a per-slot RWMutex.
// - seq: published sequence number currently stored in this slot.
// - mu: coordinates access to this slot.
// - data: payload for the sequence in this slot.
type slot[T any] struct {
	seq  uint64
	mu   sync.RWMutex
	data T
}

func (s *slot[T]) read() (T, uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data, s.seq
}

func (s *slot[T]) write(data T, seq uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	s.seq = seq
}

type cacheLinePad struct {
	_ [128 - unsafe.Sizeof(uint64(0))%128]byte
}

// Broadcast is the main fan-out structure.
// - ring: the shared ring buffer used by all senders/readers.
// - closed: atomic flag indicating broadcast has been closed.
// - subMu: protects the subs map and nextSubID.
// - subs: set of current subscriptions.
// - nextSubID: monotonically increasing subscription ID to assign to next subscriber.
type Broadcast[T any] struct {
	buffer []slot[T]
	mu     sync.Mutex
	tail   atomic.Uint64
	_      cacheLinePad

	notifyChan atomic.Value // stores chan struct{}
	_          cacheLinePad

	capacity uint64
	mask     uint64

	ctx context.Context
}

// New constructs a Broadcast with a ring buffer of at least the given capacity
// (capacity is rounded up to the next power of two).
func New[T any](ctx context.Context, capacity uint64) *Broadcast[T] {
	capacity = nextPowerOfTwo(capacity)
	b := &Broadcast[T]{
		buffer:   make([]slot[T], capacity),
		capacity: capacity,
		mask:     capacity - 1,
		ctx:      ctx,
	}
	b.tail.Store(0)
	b.notifyChan.Store(make(chan struct{}))
	return b
}

// loadNotify atomically returns the current notify channel.
// Producers close-and-swap this channel to wake all waiters.
func (b *Broadcast[T]) loadNotify() chan struct{} {
	return b.notifyChan.Load().(chan struct{})
}

// Send publishes msg to the ring and wakes subscribers.
//   - Returns ErrClosed if closed is already set (a concurrent Close after this check may still allow this Push).
//   - Push is serialised via ring’s tailMu.
//   - Wakes subscribers by iterating subs under subMu.RLock and attempting non-blocking sends on their notifyC.
func (b *Broadcast[T]) Send(msg T) error {
	select {
	case <-b.ctx.Done():
		return ErrClosed
	default:
		b.mu.Lock()
		defer b.mu.Unlock()
		tail := b.tail.Load()
		idx := tail & b.mask

		b.buffer[idx].write(msg, tail)

		b.tail.Add(1)

		// Wake all waiters by closing the current notify channel and swapping a new one.
		notify := b.loadNotify()
		close(notify)
		b.notifyChan.Store(make(chan struct{}))

		return nil
	}
}

// Subscribe adds a new subscriber starting at the current tail (next message).
// Subscription must be ended by calling Unsubscribe to terminate forwarder goroutine and cleanup resources,
// or after user facing channel is closed after draining the buffer when broadcast is closed.
//
// - Spawns a goroutine to deliver messages to sub.out.
func (b *Broadcast[T]) Subscribe(ctx context.Context) *Subscription[T] {
	sub := &Subscription[T]{
		bcast:   b,
		nextSeq: b.tail.Load(), // next msg to read
		out:     make(chan EventOrLag[T], 1),
		ctx:     ctx,
	}

	go sub.run()

	return sub
}

// nextPowerOfTwo computes the next power-of-two >= x, returning 1 for x=0.
func nextPowerOfTwo(x uint64) uint64 {
	if x == 0 {
		return 1
	}
	return 1 << uint(bits.Len64(x-1))
}
