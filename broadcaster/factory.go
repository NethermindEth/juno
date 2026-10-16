package broadcaster

import (
	"iter"

	"github.com/NethermindEth/juno/broadcaster/broadcast"
	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
	"github.com/NethermindEth/juno/feed"
)

// Kind selects the underlying pub/sub implementation a hub will use.
type Kind int

const (
	// KindFeed routes through feed.Feed: channel-based fan-out, drops on slow subscribers.
	KindFeed Kind = iota
	// KindBroadcast routes through the ring-buffer Broadcast: bounded, surfaces lag explicitly.
	KindBroadcast
)

// LagPolicy converts the ring-buffered EventOrLag stream into a plain T stream,
// deciding what to do with lag notifications (log, count, drop, ...). Each
// subscriber supplies its own at Subscribe time so different consumers of the
// same hub can apply different policies.
type LagPolicy[T any] func(iter.Seq[ring.EventOrLag[T]]) iter.Seq[T]

// defaultCapacity is the ring size used for KindBroadcast when the caller does
// not pass WithCapacity. Producers normally set their own per-stream capacity;
// this is a sane fallback (a power of two, so the ring uses it as-is).
const defaultCapacity uint64 = 128

// options holds the settings New applies.
type options struct {
	kind     Kind
	capacity uint64
}

// defaultOptions is the baseline New starts from: the feed backend with a
// reasonable ring capacity, unless an Option overrides them.
func defaultOptions() options {
	return options{kind: KindFeed, capacity: defaultCapacity}
}

// Option configures New.
type Option func(*options)

// WithKind selects the backend. Defaults to KindFeed.
func WithKind(kind Kind) Option {
	return func(c *options) { c.kind = kind }
}

// WithCapacity sets the ring capacity; only consulted for KindBroadcast.
func WithCapacity(capacity uint64) Option {
	return func(c *options) { c.capacity = capacity }
}

// New is the single access point: callers wire the kind from a flag and get back
// the same BroadcastHub[T] interface regardless of the choice.
func New[T any](opts ...Option) BroadcastHub[T] {
	cfg := defaultOptions()
	for _, opt := range opts {
		opt(&cfg)
	}
	switch cfg.kind {
	case KindFeed:
		return NewFeedAdapter(feed.New[T]())
	case KindBroadcast:
		return NewBroadcastAdapter(broadcast.New[T](cfg.capacity))
	default:
		panic("broadcaster: unknown Kind")
	}
}
