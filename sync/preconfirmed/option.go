package preconfirmed

import "time"

const (
	// DefaultPollInterval is how often the poller ticks unless overridden.
	DefaultPollInterval = 500 * time.Millisecond
	// DefaultStaleAfter is how long after a successful poll the pre-confirmed chain is served as
	// is before a read triggers a new poll, unless overridden.
	DefaultStaleAfter = 500 * time.Millisecond
	// DefaultOnDemandWait is how long a read waits for the poll it triggered before answering
	// with the stored chain, unless overridden.
	DefaultOnDemandWait = 0 * time.Second
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
