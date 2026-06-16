package client

import "time"

// WithPingConfig overrides the websocket keep-alive ping interval and the
// per-ping write/read timeout. Test-only: the production defaults are
// fine for every real RPC endpoint we've encountered, and exposing tuning
// knobs in the public API would invite cargo-culted misconfiguration.
func WithPingConfig(interval, timeout time.Duration) Option {
	return func(o *options) {
		o.pingInterval = interval
		o.pingTimeout = timeout
	}
}
