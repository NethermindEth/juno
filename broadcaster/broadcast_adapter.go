package broadcaster

import (
	"github.com/NethermindEth/juno/broadcaster/broadcast"
)

var _ BroadcastHub[any] = &BroadcastAdapter[any]{}

// BroadcastAdapter adapts broadcast.Broadcast[T] to BroadcastHub[T]. The lag
// policy lives on each Subscribable, so different consumers of the same broadcast
// can hold different responses to lag (log, count, drop, ...).
type BroadcastAdapter[T any] struct {
	broadcast *broadcast.Broadcast[T]
}

func NewBroadcastAdapter[T any](b *broadcast.Broadcast[T]) *BroadcastAdapter[T] {
	return &BroadcastAdapter[T]{broadcast: b}
}

func (a *BroadcastAdapter[T]) NewPublisher() Publisher[T] {
	return a.broadcast.NewPublisher()
}

func (a *BroadcastAdapter[T]) NewSubscribable(lagPolicy LagPolicy[T]) Subscribable[T] {
	return &broadcastSubscribable[T]{broadcast: a.broadcast, lagPolicy: lagPolicy}
}

// broadcastSubscribable carries one consumer's lag policy. Each Subscribe call
// installs that policy over a fresh ring iterator.
type broadcastSubscribable[T any] struct {
	broadcast *broadcast.Broadcast[T]
	lagPolicy LagPolicy[T]
}

func (s *broadcastSubscribable[T]) Subscribe() Subscription[T] {
	return s.broadcast.NewSubscribable().SubscribeWith(s.lagPolicy)
}
