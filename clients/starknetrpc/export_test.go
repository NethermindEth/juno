package starknetrpc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type (
	Params                  = params
	Method[P params, R any] = method[P, R]
)

var (
	Retryable = retryable
	Outcome   = outcome
)

func AssertDefaultBackoff(t *testing.T, c *Client) {
	t.Helper()
	assert.Equal(t, defaultMaxRetries, c.maxRetries)
	assert.Equal(t, defaultMinWait, c.minWait)
	assert.Equal(t, defaultMaxWait, c.maxWait)
}

func AssertCurrentTimeout(t *testing.T, c *Client, want time.Duration) {
	t.Helper()
	assert.Equal(t, want, c.timeouts.Load().GetCurrentTimeout())
}
