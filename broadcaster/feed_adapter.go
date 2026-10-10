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

// NewSubscribable returns a view over the producer feed itself; every Subscribe is a
// direct keep-last subscription, so drops are per client and nothing runs in between.
//
// lagPolicy is ignored: feed.Feed has no lag-or-event envelope to transform. The
// parameter exists only to satisfy the unified BroadcastHub interface so callers
// wired against KindBroadcast can be repointed at KindFeed without code changes.
func (a *FeedAdapter[T]) NewSubscribable(_ LagPolicy[T]) Subscribable[T] {
	return &feedSubscribable[T]{feed: a.feed}
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
