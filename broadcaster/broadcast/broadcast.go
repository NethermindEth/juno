// Package broadcast implements a fan-out event stream with overwrite-on-full semantics.
// Callers publish with [Publisher.Send], and each [Subscription] receives every message
// in order until it falls a full ring behind; it then receives a [ring.LaggedError]
// saying where to resume instead of stale data. The ring is bounded, so producers never wait
// on consumers. See [ring.RingBuffer] for the sequence and slot protocol.
package broadcast

import (
	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
)

// Broadcast is a fan-out event stream over a [ring.RingBuffer]. It mints [Publisher] and
// [Subscribable] handles; it has no lifecycle of its own, subscriptions end via
// [Subscription.Unsubscribe].
type Broadcast[T any] struct {
	ring *ring.RingBuffer[T]
}

// New constructs a [Broadcast] with a ring of at least the given capacity, rounded up to a power
// of two.
func New[T any](capacity uint64) *Broadcast[T] {
	return &Broadcast[T]{ring: ring.NewRingBuffer[T](capacity)}
}

// NewPublisher returns a [Publisher], the Send handle for this [Broadcast].
func (b *Broadcast[T]) NewPublisher() Publisher[T] {
	return Publisher[T]{ring: b.ring}
}

// NewSubscribable returns a [Subscribable], the read-only handle for creating subscriptions.
func (b *Broadcast[T]) NewSubscribable() Subscribable[T] {
	return Subscribable[T]{ring: b.ring}
}
