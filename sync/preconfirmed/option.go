package preconfirmed

import "time"

const (
	// DefaultPollInterval is how often the poller ticks unless overridden. Measured on mainnet
	// (2026-10-07), the 1s tick with the stale-after and on-demand wait below costs about 40%
	// fewer gateway requests than 500ms when idle and answers fresher at every load level,
	// because a read that finds the chain stale triggers a poll and waits for it.
	DefaultPollInterval = time.Second
	// DefaultStaleAfter is how long after a successful poll the pre-confirmed chain is served as
	// is before a read triggers a new poll, unless overridden. 250ms keeps a busy node at about
	// 2.8 gateway requests/s; 100ms or 0 polled often enough to be throttled (HTTP 429).
	DefaultStaleAfter = 250 * time.Millisecond
	// DefaultOnDemandWait is how long a read waits for the poll it triggered before answering
	// with the stored chain, unless overridden. 300ms covers nearly every poll (150-250ms end to
	// end); 150ms timed out on about one poll in five.
	DefaultOnDemandWait = 300 * time.Millisecond
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
