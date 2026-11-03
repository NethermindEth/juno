package preconfirmed

import "time"

const (
	// DefaultPollInterval is how often the poller ticks unless overridden.
	DefaultPollInterval = 500 * time.Millisecond
	// DefaultStaleAfter is how long after a successful poll the pre-confirmed chain is served as
	// is before a read triggers a new poll, unless overridden. Matching [DefaultPollInterval], the
	// ticks keep the chain fresh while they succeed, so reads trigger no polls of their own.
	DefaultStaleAfter = 500 * time.Millisecond
	// DefaultOnDemandWait is how long a read waits for the poll it triggered before answering
	// with the stored chain, unless overridden. Zero answers every read right away, and the poll
	// it triggered serves the reads after it.
	DefaultOnDemandWait time.Duration = 0
)

// options carries the optional Poller settings; see [Option].
type options struct {
	pollInterval time.Duration
	staleAfter   time.Duration
	onDemandWait time.Duration
	listener     EventListener
}

// Option is a functional option for configuring a Poller.
type Option func(*options)

// WithPollInterval overrides [DefaultPollInterval]; zero disables polling.
func WithPollInterval(interval time.Duration) Option {
	return func(o *options) { o.pollInterval = interval }
}

// WithStaleAfter overrides [DefaultStaleAfter]; zero makes every read poll.
func WithStaleAfter(staleAfter time.Duration) Option {
	return func(o *options) { o.staleAfter = staleAfter }
}

// WithOnDemandWait overrides [DefaultOnDemandWait]; zero makes reads never wait.
func WithOnDemandWait(onDemandWait time.Duration) Option {
	return func(o *options) { o.onDemandWait = onDemandWait }
}

// WithListener sets the listener every poll is reported to. By default polls are reported to a
// listener that drops them.
func WithListener(listener EventListener) Option {
	return func(o *options) { o.listener = listener }
}
