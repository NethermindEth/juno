package ring

import (
	"context"
	"sync"
)

// slot represents a single fixed-position element in the ring buffer,
// coordinating concurrent access and delivery.
//
// Fields:
//   - seq: the sequence number of the message currently stored in this slot.
//     This is a monotonically increasing counter indicating the version of data.
//     A reader compares seq with expected sequence numbers to detect
//     overwrites or message availability.
//   - mu:  a per-slot mutex that guards access to seq and data fields,
//     ensuring safe concurrent reads and writes.
//   - cond: a condition variable associated with mu, used to signal waiting
//     readers when the slot has been updated (i.e., seq advanced).
//     Readers block on cond when waiting for the next sequence to arrive.
//   - data: the payload of generic type T stored in this slot, representing
//     the event/message data for the corresponding sequence.
type Slot[T any] struct {
	seq  uint64
	mu   sync.Mutex
	cond sync.Cond
	data T
}

func NewSlot[T any]() *Slot[T] {
	slot := &Slot[T]{}
	slot.cond = *sync.NewCond(&slot.mu)
	return slot
}

func (s *Slot[T]) Read() (T, uint64) {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	return s.data, s.seq
}

func (s *Slot[T]) Write(msg T, seq uint64) {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	s.data = msg
	s.seq = seq
	s.cond.Broadcast()
}

// waitForSequence blocks until the slot holds seq or newer; ok is false once ctx is done.
// Wait cannot observe ctx, so callers must broadcast this slot after ctx is done
// (see RingBuffer.wakeWaiters).
func (s *Slot[T]) waitForSequence(
	ctx context.Context, seq uint64,
) (data T, slotSeq uint64, ok bool) {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	for s.seq < seq {
		if ctx.Err() != nil {
			var zero T
			return zero, 0, false
		}
		s.cond.Wait()
	}
	return s.data, s.seq, true
}

// wakeWaiters broadcasts on the slot to wake up any waiting readers.
func (s *Slot[T]) wakeWaiters() {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	s.cond.Broadcast()
}
