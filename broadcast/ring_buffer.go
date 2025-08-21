package broadcast

import (
	"math/bits"
	"sync"
	"sync/atomic"
)

// A single ring buffer slot with a per-slot RWMutex.
// - seq: published sequence number currently stored in this slot.
// - mu: coordinates access to this slot.
// - data: payload for the sequence in this slot.
type slot[T any] struct {
	seq  uint64
	mu   sync.RWMutex
	data T
}

// ringBuffer supports MPMC access with per-slot locks plus a global tail lock.
// - buffer: power-of-two-sized array of slots (masking index).
// - tailMu: guards tail updates and consistent reads of tail.
// - tail: next sequence to assign (1-based; newest = tail-1).
// - capacity/mask: index = (seq-1) & mask.
type ringBuffer[T any] struct {
	buffer []slot[T]
	tailMu sync.RWMutex
	tail   atomic.Uint64

	capacity uint64
	mask     uint64
}

// Rounds capacity up to the next power of two; initialises tail=1, mask=capacity-1, and allocates slots.
func newRingBuffer[T any](capacity uint64) *ringBuffer[T] {
	capacity = nextPowerOfTwo(capacity)
	rb := ringBuffer[T]{
		buffer:   make([]slot[T], capacity),
		capacity: capacity,
		mask:     capacity - 1,
	}
	rb.tail.Store(1)
	return &rb
}

// Push writes a new value:
// - Acquire tailMu (serialise writers and provide a consistent newest).
// - Lock the slot at index (tail-1)&mask; write data first, then publish seq=tail.
// - Unlock the slot, then advance tail (newest becomes tail-1), and release tailMu.
func (rb *ringBuffer[T]) Push(val T) {
	rb.tailMu.Lock()
	defer rb.tailMu.Unlock()
	tail := rb.tail.Load()
	wPos := tail - 1
	idx := wPos & rb.mask

	slot := &rb.buffer[idx]
	slot.mu.Lock()
	slot.data = val
	slot.seq = tail
	slot.mu.Unlock()
	rb.tail.Add(1)
}

// Get reads the value for sequence seq:
//   - Seq start from 1, seq == 0 returns ErrInvalidSequence.
//   - Compute slot index = (seq-1)&mask.
//   - Fast path: RLock the slot; if slot.seq == seq, read and return the slot’s data.
//   - On mismatch: RUnlock slot and acquire tailMu.RLock for a consistent newest;
//     re-check the slot after releasing slot.Mu and acquire/release tail.Mu in case a concurrent Push just published it.
//   - If seq < oldest, overwritten and return LaggedError with
//     NextSeq = newest - capacity + 1 (Oldest available).
//   - Otherwise seq > newest, return ErrFutureSeq (not yet published).
func (rb *ringBuffer[T]) Get(seq uint64) (T, error) {
	var zero T

	if seq == 0 {
		return zero, ErrInvalidSequence
	}

	tail := rb.tail.Load()
	newest := tail - 1
	if seq > newest {
		// Requested seq is ahead of newest -> not ready yet.
		return zero, ErrFutureSeq
	}

	idx := (seq - 1) & rb.mask
	slot := &rb.buffer[idx]
	slot.mu.RLock()

	if slot.seq == seq {
		// Fast-path: slot matches expected sequence.
		val := slot.data
		slot.mu.RUnlock()
		return val, nil
	}

	// Lag!: slot.seq > seq
	// Release slot.mu before acquiring tailMu,
	// otherwise concurrent producer will cause deadlock.
	slot.mu.RUnlock()

	// Check if queried seq. is overwritten.
	// We should end up this branch only on overwrite thus newest -rb.capacity should never underflow.
	oldest := newest - rb.capacity + 1

	return zero, &LaggedError{
		MissedSeq: seq,
		NextSeq:   oldest,
	}
}

// nextPowerOfTwo computes the next power-of-two >= x, returning 1 for x=0.
func nextPowerOfTwo(x uint64) uint64 {
	if x == 0 {
		return 1
	}
	return 1 << uint(bits.Len64(x-1))
}
