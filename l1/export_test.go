// NOTE: This export file should stay as is and no other additions can be done to it.
// The goal is to eventually remove each of the methods in this file by rewriting the
// tests to use the public API only.
package l1

import (
	"context"
)

// Exposes package-private surface to the external l1_test package. The
// tests live there to avoid a circular import: l1 > mocks > l1.

// Deprecated: use the public API instead.
func (c *Client) SetL1StateProvider(s L1StateProvider) { c.provider = s }

// Deprecated: use the public API instead.
func (c *Client) NonFinalisedLogs() map[uint64]*StateUpdate {
	return c.nonFinalisedLogs
}

// Deprecated: use the public API instead.
func (c *Client) SubscribeToUpdates(ctx context.Context, ch chan *StateUpdate) Subscription {
	return c.subscribeToUpdates(ctx, ch)
}

// Deprecated: use the public API instead.
func (c *Client) FinalisedHeight(ctx context.Context) (uint64, bool) {
	return c.finalisedHeight(ctx)
}
