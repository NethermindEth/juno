// Package broadcast implements a fan-out event stream with overwrite-on-full semantics.
// Multiple producers can call Send concurrently and multiple consumers can Subscribe.
// Every consumer receives every message in order unless it has fallen behind far enough
// that its next sequence has been overwritten; in that case the consumer receives a
// lag notification (LaggedError) indicating where to resume.
// Notes:
//   - Producers are serialized by a global mutex (b.mu), so producer-side throughput
//     is effectively single-writer (MPMC-safe but not truly parallel on Send).
//   - The ring is bounded and overwriting; backpressure to producers does not apply,
//     except that producers can stall when wrapping to a slot currently being read.
package broadcast

import (
	"context"
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

// Subscription represents a single consumer of the broadcast.
// - nextSeq: next sequence number to fetch from the ring.
// - out:     user-facing channel that delivers EventOrLag values.
//
// A Subscription is driven by a dedicated goroutine started by Subscribe.
// To stop receiving, cancel the context passed to Subscribe; this terminates the
// delivery goroutine and closes the 'out' channel once it exits.
//
// NOTE (semantics on close):
//   - If the Broadcast's context is canceled, producers stop. Each subscription's
//     goroutine will drain any messages already present up to the current tail,
//     then exit and close 'out'.
//   - If only the subscription context is canceled, the goroutine returns promptly
//     (without draining) and closes 'out'.
type Subscription[T any] struct {
	nextSeq uint64
	out     chan EventOrLag[T] // user-facing channel for messages
}

// run delivers messages to the subscription's out channel.
// Algorithm outline:
// - Wait until tail advances beyond nextSeq (using a broadcast-style notify channel).
// - Compute the slot index = nextSeq & mask and read the slot.
//   - If the slot's seq matches nextSeq: deliver the event and advance nextSeq.
//   - Else: the slot has been overwritten; compute the current oldest sequence and
//     emit a lag notification. Jump nextSeq to the oldest and continue.
//   - On Broadcast ctx cancellation, this goroutine drains remaining messages (until
//     nextSeq == tail at the time of checking), then exits.
//   - On subscription ctx cancellation, this goroutine exits immediately and closes out.
func (sub *Subscription[T]) run(subCtx context.Context, bcast *Broadcast[T]) {
	defer close(sub.out)

	for {
		// Wait until sub.nextSeq is published.
		// We snapshot the current notify channel once per outer iteration. Producers
		// close the channel after publishing a message and swap a new one in; observing
		// a closed notify ensures we see the tail increment (channel close establishes
		// a happens-before edge for preceding writes in Send).
		notify := bcast.loadNotify()
		if sub.nextSeq >= bcast.tail.Load() {
			// slow path, wait for next
			select {
			case <-notify:
				continue
			case <-bcast.ctx.Done():
				// Drain-on-close semantics are handled by not hitting this path.
				return
			case <-subCtx.Done():
				// Subscription canceled: stop promptly without draining.
				return
			}
		}
		// There is data available at nextSeq; try to read it
		idx := sub.nextSeq & bcast.mask
		msg, slotSeq := bcast.buffer[idx].read()
		var res EventOrLag[T]
		if slotSeq == sub.nextSeq {
			sub.nextSeq++
			res.event = msg
		} else {
			// Slot has been overwritten; compute the oldest sequence that is still
			// available and inform the subscriber to skip ahead.
			// newest and oldest are derived from the current tail. Because we only
			// reach here when overwriting has happened, newest - bcast.capacity + 1 cannot
			// underflow.
			newest := bcast.tail.Load() - 1
			oldest := newest - bcast.capacity + 1

			res.lag.MissedSeq = sub.nextSeq
			res.lag.NextSeq = oldest
			res.isLag = true

			sub.nextSeq = oldest
		}
		// Deliver the result. If the downstream is slow (out is full), this blocks
		// until either the subscriber reads or the subscription context is canceled.
		select {
		case sub.out <- res:
			continue
		case <-subCtx.Done():
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

// read returns the slot's data and sequence under a shared read lock.
// Readers are synchronized with writers to avoid tearing data/seq pairs
func (s *slot[T]) read() (T, uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data, s.seq
}

// write updates the slot's data and sequence under an exclusive write lock.
// Writers are serialized per-slot; they are also globally serialized by Broadcast.Send.
func (s *slot[T]) write(data T, seq uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	s.seq = seq
}

// Broadcast is the main fan-out structure.
// Fields:
//   - buffer: ring buffer of slots.
//   - mu:     global mutex serializing all Send operations across producers.
//   - tail:   next sequence to assign; number of messages published so far.
//   - notifyChan: atomic holder for a "wake-up" channel. Each Send closes the current
//     channel to wake all waiters and swaps in a new channel for future sends.
//   - capacity: ring capacity (power of two).
//   - mask:     capacity-1 for index masking.
//   - ctx:      context signaling Broadcast lifetime; when canceled, Send returns ErrClosed
//     and subscriptions drain outstanding messages before terminating.
type Broadcast[T any] struct {
	buffer []slot[T]
	mu     sync.Mutex
	tail   atomic.Uint64

	notifyChan atomic.Value // stores chan struct{}

	capacity uint64
	mask     uint64

	ctx context.Context
}

// New constructs a Broadcast with a ring of at least the given capacity.
// The actual capacity is rounded up to the next power of two.
// A fresh notify channel is created and stored so that waiters can select on it.
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
// Producers close-and-swap this channel to wake all subscribers waiting for new data.
// Receiving from a closed channel establishes a happens-before relationship with
// the Send that performed the close, ensuring tail advancement is visible.
func (b *Broadcast[T]) loadNotify() chan struct{} {
	return b.notifyChan.Load().(chan struct{})
}

// Send publishes msg to the ring and wakes subscribers.
// Behavior:
// - Returns ErrClosed if the Broadcast context is done at entry.
// - Serializes all producers via b.mu; Send is effectively single-writer.
// - Writes the message and its sequence into the computed slot.
// - Increments tail to point to the next sequence to be written.
// - Wakes all waiters by closing the current notify channel and swapping in a new one.
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

// Subscribe creates a new subscription that starts from the current tail (i.e., it
// will receive the next message published). The returned Subscription spawns an
// internal delivery goroutine. To stop receiving, cancel the context; the out
// channel is closed on termination.
func (b *Broadcast[T]) Subscribe(ctx context.Context) *Subscription[T] {
	sub := &Subscription[T]{
		nextSeq: b.tail.Load(), // next msg to read
		out:     make(chan EventOrLag[T], 1),
	}

	go sub.run(ctx, b)

	return sub
}

// nextPowerOfTwo computes the next power-of-two >= x, returning 1 for x=0.
func nextPowerOfTwo(x uint64) uint64 {
	if x == 0 {
		return 1
	}
	return 1 << uint(bits.Len64(x-1))
}
