package feeder

import (
	"net/http"
	"time"

	"github.com/NethermindEth/juno/utils/log"
)

// options holds configuration for constructing a feeder client.
type options struct {
	httpClient *http.Client
	backoff    Backoff
	maxRetries int
	maxWait    time.Duration
	minWait    time.Duration
	logger     log.StructuredLogger
	userAgent  string
	apiKey     string
	listener   EventListener
	timeouts   *Timeouts
}

// Option is a functional option for configuring the feeder client.
type Option func(*options)

func WithListener(l EventListener) Option {
	return func(o *options) { o.listener = l }
}

func WithBackoff(b Backoff) Option {
	return func(o *options) { o.backoff = b }
}

func WithMaxRetries(num int) Option {
	return func(o *options) { o.maxRetries = num }
}

func WithMaxWait(d time.Duration) Option {
	return func(o *options) { o.maxWait = d }
}

func WithMinWait(d time.Duration) Option {
	return func(o *options) { o.minWait = d }
}

func WithLogger(logger log.StructuredLogger) Option {
	return func(o *options) { o.logger = logger }
}

func WithUserAgent(ua string) Option {
	return func(o *options) { o.userAgent = ua }
}

func WithAPIKey(key string) Option {
	return func(o *options) { o.apiKey = key }
}

func WithHTTPClient(client *http.Client) Option {
	return func(o *options) { o.httpClient = client }
}

func WithTimeouts(timeouts []time.Duration, fixed bool) Option {
	return func(o *options) { o.timeouts = makeTimeouts(timeouts, fixed) }
}

// requestConfig holds per-request overrides of the client's retry behavior.
type requestConfig struct {
	failFastOnBadRequest bool
}

// requestOption is a functional option applied to a single request.
type requestOption func(*requestConfig)

// failFastOnBadRequest makes get return an HTTP 400 immediately instead of
// burning the retry budget. A 400 is deterministic — retrying cannot help.
func failFastOnBadRequest() requestOption {
	return func(c *requestConfig) { c.failFastOnBadRequest = true }
}
