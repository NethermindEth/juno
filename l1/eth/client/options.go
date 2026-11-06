// Package client speaks JSON-RPC 2.0 to an Ethereum execution-layer node
// over WebSocket. It implements the small surface juno needs to follow the
// L1 head and serve starknet_getMessageStatus.
package client

import (
	"time"

	"github.com/NethermindEth/juno/utils/log"
)

type Option func(*options)

type options struct {
	logger       log.StructuredLogger
	pingInterval time.Duration
	pingTimeout  time.Duration
	dialTimeout  time.Duration
}

func WithLogger(l log.StructuredLogger) Option {
	return func(o *options) { o.logger = l }
}

func WithDialTimeout(d time.Duration) Option {
	return func(o *options) { o.dialTimeout = d }
}

// WithPingConfig falls back to the defaults (30s/10s) for non-positive values.
func WithPingConfig(interval, timeout time.Duration) Option {
	return func(o *options) {
		o.pingInterval = interval
		o.pingTimeout = timeout
	}
}
