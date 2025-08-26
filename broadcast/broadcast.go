// Package broadcast implements a fan-out event stream with overwrite-on-full semantics.
// Multiple producers can call Send concurrently and multiple consumers can Subscribe.
// Every consumer receives every message in order unless it has fallen behind far enough
// that its next sequence has been overwritten; in that case the consumer receives a
// lag notification (LaggedError) indicating where to resume.
// Notes:
//   - Producers are serialised by a global mutex (b.mu), so producer-side throughput
//     is effectively single-writer (MPMC-safe but not truly parallel on Send).
//   - The ring is bounded and overwriting; backpressure to producers does not apply,
//     except that producers can stall when wrapping to a slot currently being read.
package broadcast

import (
	"math/bits"
	"sync"
	"sync/atomic"
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
		return zero, ErrNoEvent
	}
	return e.event, nil
}

// Lag returns the lag info or an error if none.
func (e *EventOrLag[T]) Lag() (LaggedError, error) {
	if !e.isLag {
		return LaggedError{}, ErrNoLag
	}
	return e.lag, nil
}

// Subscription represents a single consumer.
// Fields:
// - bcast: back reference to broadcast
// - nextSeq: next sequence number this subscriber expects to receive.
// - out:     user-facing channel delivering EventOrLag values.
// - done:    internal stop signal for this subscription.
//
// Lifecycle:
//   - Created by Broadcast.Subscribe, which spawns a delivery goroutine (run).
//   - Call Unsubscribe to stop; it requests the goroutine to exit promptly and
//     wakes it if blocked, then the goroutine closes out upon return.
//   - If Broadcast.Close is called, subscriptions will only exit once
//     they can observe the closed context in their run loop, after draining the channel.
type Subscription[T any] struct {
	bcast     *Broadcast[T]
	nextSeq   atomic.Uint64
	out       chan EventOrLag[T] // user-facing channel for messages
	unsubOnce sync.Once
	done      chan struct{}
}

// run delivers messages to the subscription's out channel.

// Algorithm Outline:
// - Compute the ring slot index for the next expected sequence (idx = nextSeq & mask).
// - Acquire that slot’s lock and read slot.seq.
//   - While slot.seq <= nextSeq: wait on that slot’s cond, unless Unsubscribe or
//     Broadcast context cancellation is detected before blocking.
//   - Once slot.seq > nextSeq, copy slot.data while holding the lock and release it.
//   - If slot.seq == nextSeq+1, deliver the event and advance nextSeq by 1.
//   - Otherwise, the slot has been overwritten at least once since nextSeq+1; compute
//     the oldest available sequence from the current tail and deliver a lag
//     notification with MissedSeq=prevNextSeq and NextSeq=oldest, then jump nextSeq to oldest.
//   - Deliver the result to sub.out; if sub.out is full, block until either the value
//     is delivered or Unsubscribe is requested (done is closed).
func (sub *Subscription[T]) run() {
	defer close(sub.out)
	bcast := sub.bcast
	for {
		nextSeq := sub.nextSeq.Load()

		idx := nextSeq & bcast.mask
		slot := &bcast.buffer[idx]

		slot.cond.L.Lock()
		slotSeq := slot.seq
		for slotSeq <= nextSeq {
			select {
			case <-sub.done:
				slot.cond.L.Unlock()
				return
			case <-bcast.done:
				slot.cond.L.Unlock()
				return
			default:
				slot.cond.Wait()
				slotSeq = slot.seq
			}
		}
		msg := slot.data
		slot.cond.L.Unlock()

		var res EventOrLag[T]
		if slotSeq == nextSeq+1 {
			sub.nextSeq.Add(1)
			res.event = msg
		} else {
			// Slot has been overwritten; compute the oldest sequence that is still
			// available and inform the subscriber to skip ahead.
			// newest and oldest are derived from the current tail. Because we only
			// reach here when overwriting has happened, newest - bcast.capacity + 1 cannot
			// underflow.
			tail := bcast.tail.Load()
			newest := tail - 1
			oldest := newest - bcast.capacity + 1

			res.lag.MissedSeq = nextSeq
			res.lag.NextSeq = oldest
			res.isLag = true

			sub.nextSeq.Store(oldest)
		}
		// Deliver the result. If the downstream is slow (out is full), this blocks
		// until either the subscriber reads or the subscription context is canceled.
		select {
		case sub.out <- res:
			continue
		case <-sub.done:
			return
		}
	}
}

// Recv returns the user-facing channel for this subscription.
func (sub *Subscription[T]) Recv() <-chan EventOrLag[T] {
	return sub.out
}

// Unsubscribe terminates the subscribers delivery goroutine.
func (sub *Subscription[T]) Unsubscribe() {
	sub.unsubOnce.Do(func() {
		close(sub.done)
		// wake if subscriber is waiting
		bcast := sub.bcast
		idx := sub.nextSeq.Load() & bcast.mask
		slot := &bcast.buffer[idx]
		slot.cond.L.Lock()
		slot.cond.Broadcast()
		slot.cond.L.Unlock()
	})
}

// A single ring buffer slot with a per-slot RWMutex.
// - seq: published sequence number currently stored in this slot.
// - mu: coordinates access to this slot.
// - data: payload for the sequence in this slot.
type slot[T any] struct {
	seq  uint64
	mu   sync.Mutex
	cond sync.Cond
	data T
}

// Broadcast is the fan-out ring buffer.
// Fields:
// - buffer:   ring buffer of slots.
// - mu:       global mutex serialising all Send calls (MPMC-safe, single-writer).
// - tail:     next sequence to assign (number of messages published so far).
// - capacity: ring capacity (power of two, rounded up from the requested value).
// - mask:     capacity-1 for index masking.
// - ctx:      broadcast lifetime context; when canceled, Send returns ErrClosed.
// - cancel:   function to cancel ctx
type Broadcast[T any] struct {
	buffer []slot[T]
	mu     sync.Mutex
	tail   atomic.Uint64

	capacity uint64
	mask     uint64

	closeOnce sync.Once
	done      chan struct{}
}

// New constructs a Broadcast with a ring of at least the given capacity.
// - The actual capacity is rounded up to the next power of two.
// - Each slot’s condition variable is initialised to use the slot’s mutex.
// - The initial tail is 0 (no messages published).
func New[T any](capacity uint64) *Broadcast[T] {
	capacity = nextPowerOfTwo(capacity)
	rb := make([]slot[T], capacity)
	for i := range rb {
		slot := &rb[i]
		slot.cond = *sync.NewCond(&slot.mu)
	}
	b := &Broadcast[T]{
		buffer:   rb,
		capacity: capacity,
		mask:     capacity - 1,
		done:     make(chan struct{}),
	}
	b.tail.Store(0)
	return b
}

// Send publishes msg to the ring.
// Behaviour:
//   - Returns ErrClosed if the Broadcast context is already canceled.
//   - Serialises concurrent producers via b.mu.
//   - Writes msg to slot at index (tail & mask), sets slot.seq to tail+1, and
//     broadcasts on that slot’s condition variable to wake any waiters on that slot.
//   - Increments tail by 1.
//   - No backpressure: Send does not wait for readers (aside from brief acquisition
//     of the per-slot lock).
func (b *Broadcast[T]) Send(msg *T) error {
	// Acquire lock upfront to avoid races around Close.
	b.mu.Lock()
	defer b.mu.Unlock()

	select {
	case <-b.done:
		return ErrClosed
	default:
		tail := b.tail.Load()
		idx := tail & b.mask
		slot := &b.buffer[idx]

		slot.cond.L.Lock()
		slot.data = *msg
		slot.seq = tail + 1
		b.tail.Add(1)
		slot.cond.Broadcast()
		slot.cond.L.Unlock()

		return nil
	}
}

// Subscribe creates a new subscription that starts from the current tail (i.e., it
// will receive the next message published). The returned Subscription spawns an
// internal delivery goroutine. To stop receiving, call Unsubscribe; the out
// channel is closed on termination.
func (b *Broadcast[T]) Subscribe() *Subscription[T] {
	sub := &Subscription[T]{
		out:   make(chan EventOrLag[T], 1),
		done:  make(chan struct{}),
		bcast: b,
	}
	sub.nextSeq.Store(b.tail.Load())

	go sub.run()

	return sub
}

// Close cancels the Broadcast context (preventing further successful Send calls).
// It then broadcasts on the cond of the slot at index (tail & mask) to wake any
// waiters on that slot. Subscribers only waits for tail thus only broadcasting to tail slot
// should wake all waiting goroutines. Closes under global lock to avoid race with producers
// in order to preserve the integrity of single slot broadcast closing.
func (b *Broadcast[T]) Close() {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		close(b.done)
		tail := b.tail.Load()
		idx := tail & b.mask
		// Wake all waiters
		slot := &b.buffer[idx]
		slot.cond.L.Lock()
		slot.cond.Broadcast()
		slot.cond.L.Unlock()
	})
}

// nextPowerOfTwo computes the next power-of-two >= x, returning 1 for x=0.
func nextPowerOfTwo(x uint64) uint64 {
	if x == 0 {
		return 1
	}
	return 1 << uint(bits.Len64(x-1))
}
