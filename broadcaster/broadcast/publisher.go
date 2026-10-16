package broadcast

import (
	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
)

// Publisher is a Send handle for a [Broadcast]. It is a value type and safe to copy.
type Publisher[T any] struct {
	ring *ring.RingBuffer[T]
}

// Send publishes msg to the ring; it never blocks on readers beyond the per-slot lock.
//
// T is passed by value: callers use pointer or interface element types (e.g.
// *core.Block), so this is a single-word copy and avoids a pointer-to-pointer.
func (p Publisher[T]) Send(msg T) {
	p.ring.Write(msg)
}
