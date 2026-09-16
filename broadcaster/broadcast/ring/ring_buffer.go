// Package ring implements the MPMC overwrite-on-full ring buffer that backs the
// broadcast package: writers publish under one mutex, readers block only on the
// per-slot lock of the slot they need and are told when they have been lapped.
package ring

import (
	"context"
	"iter"
	"math/bits"
	"sync"
	"sync/atomic"
)

// RingBuffer is a multi-producer multi-consumer ring buffer with overwrite-on-full
// semantics: Write never blocks on readers and never fails, it overwrites the oldest
// slot once the ring is full, and a reader that has fallen a full capacity behind
// observes a lag notification rather than stale data.
//
// Every message gets a monotonically increasing sequence number and lands in slot
// seq & mask. Writes are serialised by mu so the tail read, slot write and tail
// increment are one atomic step; that is the only global lock, and readers never
// take it. Readers wait on the per-slot condition variable of the slot they need
// (see Iterator and Slot.waitForSequence), so a Write only wakes readers of the
// slot it filled.
//
// Fields:
//   - buffer:   ring of slots.
//   - mu:       serialises Write calls.
//   - tail:     sequence of the last published message (0 before the first Write).
//   - capacity: ring capacity, a power of two rounded up from the requested value.
//   - mask:     capacity-1 for index masking.
//
// RingBuffer has no lifecycle of its own. Cancelling the ctx passed to Iterator ends
// that reader; nothing ends the ring (see wakeWaiters).
type RingBuffer[T any] struct {
	buffer   []Slot[T]
	mu       sync.Mutex // serializes Write calls
	tail     atomic.Uint64
	capacity uint64
	mask     uint64
}

// NewRingBuffer constructs an empty RingBuffer whose capacity is the given value rounded up to a
// power of two.
func NewRingBuffer[T any](capacity uint64) *RingBuffer[T] {
	capacity = nextPowerOfTwo(capacity)
	rb := make([]Slot[T], capacity)
	for i := range rb {
		slot := &rb[i]
		slot.cond = *sync.NewCond(&slot.mu)
	}
	return &RingBuffer[T]{
		buffer:   rb,
		capacity: capacity,
		mask:     capacity - 1,
	}
}

// Write publishes msg at sequence Tail()+1, overwriting whatever the slot held, and
// returns that sequence. Concurrent calls are serialised by rb.mu.
func (rb *RingBuffer[T]) Write(msg T) uint64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	tail := rb.tail.Add(1)
	rb.Slot(tail).Write(msg, tail)
	return tail
}

// Slot returns the slot that holds, or will hold, the given sequence.
func (rb *RingBuffer[T]) Slot(seq uint64) *Slot[T] {
	idx := seq & rb.mask
	return &rb.buffer[idx]
}

// Tail returns the sequence of the last published message; 0 before the first Write.
func (rb *RingBuffer[T]) Tail() uint64 {
	return rb.tail.Load()
}

// Capacity returns the ring capacity, a power of two.
func (rb *RingBuffer[T]) Capacity() uint64 {
	return rb.capacity
}

// wakeWaiters is the Iterator's cancellation hook: it broadcasts the slot after the tail,
// the only slot a sleeping reader can be on that no in-flight Write will broadcast.
//
// Why one broadcast is enough. cancel stores ctx.Err before AfterFunc spawns this callback,
// and waitForSequence checks ctx.Err under the slot lock right before Wait, so once this
// runs no reader can start sleeping. A reader already asleep when the tail t is read sits
// on some slot k <= t+1, because nextSeq never exceeds tail+1. k == t+1 is woken here.
// k <= t belongs to a Write that has already bumped the tail and broadcasts slot k as its
// next step. Either way the reader wakes, re-checks ctx.Err and returns.
//
// Taking rb.mu here would freeze the tail and simplify the argument, but it would stall
// the producer once per unsubscribe; readers and this hook only ever take slot locks.
func (rb *RingBuffer[T]) wakeWaiters() {
	rb.Slot(rb.Tail() + 1).wakeWaiters()
}

// Iterator returns an iter.Seq that yields EventOrLag[T] values from the ring buffer.
// It handles waiting for messages, detecting lag, and cancellation.
// The sequence terminates when the context is cancelled or when yield returns false.
//
// The subscription point is pinned at call time, so Iterator returns a single-use
// stream; call Iterator again for a fresh subscription.
//
// Cancellation cannot interrupt cond.Wait, so an AfterFunc broadcasts the tail slot
// once ctx is done; waitForSequence re-checks ctx under the slot lock before each Wait
// (see wakeWaiters for why that cannot strand a reader).
func (rb *RingBuffer[T]) Iterator(ctx context.Context) iter.Seq[EventOrLag[T]] {
	nextSeq := rb.Tail() + 1

	return func(yield func(EventOrLag[T]) bool) {
		stop := context.AfterFunc(ctx, rb.wakeWaiters)
		defer stop()

		for {
			msg, slotSeq, ok := rb.Slot(nextSeq).waitForSequence(ctx, nextSeq)
			if !ok {
				return
			}

			var result EventOrLag[T]
			if slotSeq != nextSeq {
				// Slot has been overwritten; resume at the oldest sequence still in the
				// ring. The lag's NextSeq is what we will actually deliver next.
				oldest := rb.Tail() - rb.Capacity() + 1
				result = NewLag[T](nextSeq, oldest)
				nextSeq = oldest
			} else {
				result = NewEvent(msg)
				nextSeq++
			}

			if !yield(result) {
				return
			}
		}
	}
}

// nextPowerOfTwo computes the next power-of-two >= x, returning 1 for x=0.
func nextPowerOfTwo(x uint64) uint64 {
	if x == 0 {
		return 1
	}
	return 1 << uint(bits.Len64(x-1))
}
