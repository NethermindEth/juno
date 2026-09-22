package ring

import (
	"context"
	"sync"
)

// Slot represents a single fixed-position element in the ring buffer,
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

func (s *Slot[T]) Read() (T, uint64) {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	return s.data, s.seq
}

// Write stores msg under seq and wakes waiters. A lapped writer, whose slot already holds
// a newer sequence, is dropped without a broadcast: the newer write already woke everyone,
// and any reader wanting the dropped sequence observes the lap instead of sleeping.
func (s *Slot[T]) Write(msg T, seq uint64) {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	if seq <= s.seq {
		return
	}
	s.data = msg
	s.seq = seq
	s.cond.Broadcast()
}

// waitForSequence blocks until the slot holds seq or newer, or ctx is done, in which case
// it returns ctx.Err(). Wait cannot observe ctx, so callers must broadcast this slot after
// ctx is done (see RingBuffer.wakeWaiters).
func (s *Slot[T]) waitForSequence(ctx context.Context, seq uint64) (T, uint64, error) {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, 0, err
		}
		if s.seq >= seq {
			return s.data, s.seq, nil
		}
		s.cond.Wait()
	}
}

// wakeWaiters broadcasts on the slot to wake up any waiting readers.
func (s *Slot[T]) wakeWaiters() {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	s.cond.Broadcast()
}
