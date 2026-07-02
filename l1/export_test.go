package l1

import (
	"context"
)

// Exposes package-private surface to the external l1_test package. The
// tests live there to avoid a circular import: l1 -> mocks -> l1.

// SetSettlement swaps the underlying settlement layer. Used by tests
// that rebuild expectations across iterations on a single Client.
func (c *Client) SetSettlement(s SettlementLayer) { c.settlement = s }

// NonFinalisedLogs exposes the in-memory cache of pre-finality state
// updates for assertions about cache partitioning and reorg handling.
func (c *Client) NonFinalisedLogs() map[uint64]*StateUpdate {
	return c.nonFinalisedLogs
}

// SubscribeToUpdates exposes the resubscribe retry loop for testing
// its interaction with context cancellation and resubscribeDelay.
func (c *Client) SubscribeToUpdates(ctx context.Context, ch chan *StateUpdate) Subscription {
	return c.subscribeToUpdates(ctx, ch)
}

// FinalisedHeight exposes the inner retry loop driven by setL1Head.
func (c *Client) FinalisedHeight(ctx context.Context) (uint64, bool) {
	return c.finalisedHeight(ctx)
}
