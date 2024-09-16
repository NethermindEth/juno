package utils

import (
	"context"
	"fmt"
	"time"

	"tailscale.com/logtail/backoff"
)

// Backoff backs off when err != nil and returns prematurely when the context is cancelled.
// It is stateful: a single instance should be used for all requests made to a given endpoint.
type Backoff interface {
	BackOff(ctx context.Context, err error)
}

func NewBackoff(name string, log SimpleLogger, maxBackoff time.Duration) *backoff.Backoff {
	return backoff.NewBackoff(name, func(format string, args ...any) {
		log.Warnw(fmt.Sprintf(format, args...))
	}, maxBackoff)
}

type nopBackoff struct{}

func (nb nopBackoff) BackOff(_ context.Context, _ error) {}

func NewNopBackoff() Backoff {
	return nopBackoff{}
}
