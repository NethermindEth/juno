// Package broadcast implements a fan-out event stream with overwrite-on-full semantics.
// Multiple Publishers can call Send concurrently and multiple consumers can Subscribe.
// Every consumer receives every message in order unless it has fallen behind far enough
// that its next sequence has been overwritten; in that case the consumer receives a
// lag notification (LaggedError) indicating where to resume.
//
// Architecture:
//   - ring.RingBuffer: slot storage, sequence allocation and the cancellable read iterator
//   - Broadcast (this file): owns the ring and mints the two handles below
//   - Publisher (publisher.go): write handle, Send
//   - Subscribable / Subscription (subscription.go): read handles that pump the ring
//     iterator into a channel
//
// Design Philosophy:
//
//	The package is optimized for performance over extensibility. The ring buffer's
//	slot-based design with condition variables is tightly coupled to its iterator.
//	This coupling is intentional - it enables zero-copy reads and efficient wake-up.
//
//	What's replaceable:
//	  - Interface wrappers (middleware, adapters, lag policies)
//
//	What's not replaceable (performance-critical):
//	  - Storage backend (RingBuffer's slot structure)
//	  - Write path (atomic sequence claim, then the slot's own lock)
//	  - Reading logic (RingBuffer.Iterator, tightly coupled to slot structure)
//
//	For different storage or delivery patterns, consider implementing a different
//	Broadcaster (see the feed package for a channel-based alternative).
//
// Notes:
//   - Producers run in parallel: Send claims a sequence with one atomic increment and
//     takes only the lock of the slot it fills. A producer lapped before it stored is
//     dropped, which overwrite-on-full already implies.
//   - The ring is bounded and overwriting; backpressure to producers does not apply,
//     except that a producer can stall briefly on a slot lock held by a reader.
package broadcast

import (
	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
)

// Broadcast is a fan-out event stream over a RingBuffer. It mints Publishers and
// Subscribables; it has no lifecycle of its own, subscriptions end via Unsubscribe.
type Broadcast[T any] struct {
	ring *ring.RingBuffer[T]
}

// New constructs a Broadcast with a ring of at least the given capacity, rounded up to a power
// of two.
func New[T any](capacity uint64) *Broadcast[T] {
	return &Broadcast[T]{ring: ring.NewRingBuffer[T](capacity)}
}

// NewPublisher returns a Send handle for this Broadcast.
func (b *Broadcast[T]) NewPublisher() Publisher[T] {
	return Publisher[T]{ring: b.ring}
}

// NewSubscribable returns a read-only handle for creating subscriptions.
func (b *Broadcast[T]) NewSubscribable() Subscribable[T] {
	return Subscribable[T]{ring: b.ring}
}
