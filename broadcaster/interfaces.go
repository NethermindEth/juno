// Package broadcaster provides a unified interface for pub/sub implementations.
// It defines common interfaces that both broadcast and feed packages can implement.
//
// Architecture:
//   - BroadcastHub: Factory that creates Publishers and Subscribables
//   - Publisher: For sending messages to all subscribers
//   - Subscribable: Read-only view bound to a single lag policy
//   - Subscription: For receiving messages from a subscription
//
// Type parameter T is the message type being published.
//
// Lag handling: when the underlying implementation can detect that a subscriber
// fell behind (broadcast/ring), the lag policy supplied at NewSubscribable time
// decides what to do — log, count, panic, etc. Each consumer asks the hub for its
// own Subscribable with its own policy, so the same hub can serve subscribers
// with different lag responses simultaneously.
package broadcaster

// SubscribableSource mints Subscribables with per-consumer lag policies. Producers
// hand this out to consumers that must be able to subscribe but not publish.
type SubscribableSource[T any] interface {
	// NewSubscribable creates a read-only view for creating subscriptions.
	// The lagPolicy transforms the underlying lag-aware stream into plain T values
	// and is the consumer's hook for handling lag (log, count, panic, ...).
	// Implementations that cannot detect lag (e.g. feed) ignore lagPolicy; the ring
	// backend requires it, so every consumer states its lag response explicitly.
	NewSubscribable(lagPolicy LagPolicy[T]) Subscribable[T]
}

// BroadcastHub is a factory that creates Publishers and Subscribables.
type BroadcastHub[T any] interface {
	SubscribableSource[T]

	// NewPublisher creates a new Publisher for sending messages.
	NewPublisher() Publisher[T]
}

// Publisher defines the interface for sending messages to subscribers.
//
// T is passed to Send by value. Callers use pointer or interface element types
// (e.g. *core.Block, core.Transaction), so this is a single-word copy — there is
// no *T and thus no pointer-to-pointer to allocate.
type Publisher[T any] interface {
	// Send publishes a message to all subscribers.
	Send(msg T)
}

// Subscribable defines a read-only interface for creating subscriptions.
type Subscribable[T any] interface {
	// Subscribe creates a new subscription that delivers T values.
	Subscribe() Subscription[T]
}

// Subscription defines the interface for receiving messages from a subscription.
type Subscription[T any] interface {
	// Recv returns the channel for receiving messages.
	// The channel is closed when the subscription is terminated.
	Recv() <-chan T

	// Unsubscribe terminates the subscription and closes the channel. Idempotent.
	Unsubscribe()
}
