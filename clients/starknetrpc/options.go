package starknetrpc

import "time"

const (
	defaultMaxRetries = 10
	defaultMaxWait    = 2 * time.Second
	defaultMinWait    = 500 * time.Millisecond
)

type options struct {
	maxRetries int
	maxWait    time.Duration
	minWait    time.Duration
	userAgent  string
	listener   EventListener
}

type Option func(*options)

func defaultOptions() options {
	return options{
		maxRetries: defaultMaxRetries,
		maxWait:    defaultMaxWait,
		minWait:    defaultMinWait,
		userAgent:  "",
		listener:   &SelectiveListener{OnResponseCb: nil},
	}
}

func WithListener(l EventListener) Option {
	return func(o *options) { o.listener = l }
}

func WithUserAgent(ua string) Option {
	return func(o *options) { o.userAgent = ua }
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
