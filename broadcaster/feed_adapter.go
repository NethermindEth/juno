package broadcaster

import (
	"github.com/NethermindEth/juno/feed"
)

var _ BroadcastHub[any] = &FeedAdapter[any]{}

// FeedAdapter adapts feed.Feed to implement BroadcastHub[T].
type FeedAdapter[T any] struct {
	feed *feed.Feed[T]
}

func NewFeedAdapter[T any](feed *feed.Feed[T]) *FeedAdapter[T] {
	return &FeedAdapter[T]{feed: feed}
}

func (a *FeedAdapter[T]) NewPublisher() Publisher[T] {
	return &feedPublisher[T]{feed: a.feed}
}

// NewSubscribable returns a fan-out view Tee'd from the producer feed: it opens a
// single upstream subscription and re-fans-out to this Subscribable's own
// subscribers through an internal feed, keeping the producer's Send O(1) in
// subscriber count (the O(N) fan-out runs on the Tee goroutine).
//
// Each call creates its own Tee and upstream subscription, so a caller that wants
// one upstream subscription must reuse the returned Subscribable across its
// consumers rather than calling NewSubscribable per consumer.
//
// lagPolicy is ignored: feed.Feed has no lag-or-event envelope to transform. The
// parameter exists only to satisfy the unified BroadcastHub interface so callers
// wired against KindBroadcast can be repointed at KindFeed without code changes.
func (a *FeedAdapter[T]) NewSubscribable(_ LagPolicy[T]) Subscribable[T] {
	internal := feed.New[T]()
	feed.Tee(a.feed.SubscribeKeepLast(), internal)
	return &feedSubscribable[T]{feed: internal}
}

type feedPublisher[T any] struct {
	feed *feed.Feed[T]
}

func (p *feedPublisher[T]) Send(msg T) {
	p.feed.Send(msg)
}

type feedSubscribable[T any] struct {
	feed *feed.Feed[T]
}

func (s *feedSubscribable[T]) Subscribe() Subscription[T] {
	return s.feed.SubscribeKeepLast()
}
