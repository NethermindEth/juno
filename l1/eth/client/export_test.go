package client

import (
	"time"

	"github.com/NethermindEth/juno/l1/eth"
)

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

// CancelPendingFor invokes wsTransport.cancelPending against an existing
// subscription, simulating the race window in callWithSubReg where
// dispatchResponse registered the sub (pendingSub.id is set) just before
// the caller's ctx fired. From the public surface this race is timing-
// dependent and not reachable deterministically — once the server's
// response has been processed into pendingSub.id, it has also already
// been delivered on the reply channel, so the caller's select picks
// pseudo-randomly between ctx.Done() and the ready reply.
//
// Test-only: production callers reach cancelPending through callWithSubReg
// when their ctx fires; tests use this helper to exercise the
// leaked-subscription cleanup branch deterministically.
func CancelPendingFor(c *Client, sub eth.Subscription) {
	s := sub.(*wsLogSub)
	// id 0 is fine — the pending/pendingSubs entries for the original
	// subscribe call have already been removed by dispatchResponse;
	// only the t.subs cleanup + best-effort eth_unsubscribe matter here.
	c.tr.cancelPending(0, s)
}
