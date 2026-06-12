package l1

import (
	"context"

	"github.com/NethermindEth/juno/l1/eth"
)

// This file is compiled only with the test binary. It exposes
// package-private surface to the external l1_test package, which is
// where the L1 client tests live (separating them avoids a circular
// import: l1 -> mocks -> l1).

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
func (c *Client) SubscribeToUpdates(ctx context.Context, ch chan *StateUpdate) eth.Subscription {
	return c.subscribeToUpdates(ctx, ch)
}

// FinalisedHeight exposes the inner retry loop driven by setL1Head.
func (c *Client) FinalisedHeight(ctx context.Context) (uint64, bool) {
	return c.finalisedHeight(ctx)
}
