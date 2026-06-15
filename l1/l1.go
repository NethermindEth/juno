package l1

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

type Client struct {
	settlement            SettlementLayer
	l2Chain               *blockchain.Blockchain
	logger                log.StructuredLogger
	network               *networks.Network
	resubscribeDelay      time.Duration
	pollFinalisedInterval time.Duration
	catchUpChunkSize      uint64
	nonFinalisedLogs      map[uint64]*StateUpdate
	listener              EventListener
}

var _ service.Service = (*Client)(nil)

// defaultCatchUpChunkSize is the L1 block range per backward eth_getLogs request.
const defaultCatchUpChunkSize uint64 = 1_000

// options holds configuration for constructing a l1 client.
type options struct {
	EventListener         EventListener
	ResubscribeDelay      time.Duration
	PollFinalisedInterval time.Duration
	// CatchUpChunkSize is the L1 block range per backward eth_getLogs request
	// during the startup catch-up scan.
	CatchUpChunkSize uint64
}

// Option is a functional option for configuring l1 client options.
type Option func(*options)

func WithEventListener(l EventListener) Option {
	return func(o *options) { o.EventListener = l }
}

func WithResubscribeDelay(d time.Duration) Option {
	return func(o *options) { o.ResubscribeDelay = d }
}

// WithPollFinalisedInterval sets the time to wait before
// checking for an update to the finalised L1 block.
func WithPollFinalisedInterval(d time.Duration) Option {
	return func(o *options) { o.PollFinalisedInterval = d }
}

// WithCatchUpChunkSize sets the L1 block range per backward eth_getLogs request
// during the startup catch-up scan.
func WithCatchUpChunkSize(size uint64) Option {
	return func(o *options) { o.CatchUpChunkSize = size }
}

func NewClient(
	settlement SettlementLayer,
	chain *blockchain.Blockchain,
	logger log.StructuredLogger,
	opts ...Option,
) *Client {
	o := options{
		EventListener:         SelectiveListener{},
		ResubscribeDelay:      10 * time.Second,
		PollFinalisedInterval: time.Minute,
		CatchUpChunkSize:      defaultCatchUpChunkSize,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &Client{
		settlement:            settlement,
		l2Chain:               chain,
		logger:                logger,
		network:               chain.Network(),
		resubscribeDelay:      o.ResubscribeDelay,
		pollFinalisedInterval: o.PollFinalisedInterval,
		catchUpChunkSize:      o.CatchUpChunkSize,
		nonFinalisedLogs:      make(map[uint64]*StateUpdate),
		listener:              o.EventListener,
	}
}

// subscribeToUpdates blocks until a subscription is established. If context is cancelled,
// returns nil
func (c *Client) subscribeToUpdates(
	ctx context.Context,
	updateChan chan *StateUpdate,
) eth.Subscription {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Debug(
				"Program stopped before resubscription was successful",
				zap.Error(ctx.Err()),
			)
			return nil
		case <-timer.C:
			updateSub, err := c.settlement.WatchStateUpdate(ctx, updateChan)
			if err == nil {
				return updateSub
			}
			c.logger.Debug(
				"Failed to subscribe to L1 state updates",
				zap.Duration("tryAgainIn", c.resubscribeDelay),
				zap.Error(err),
			)
			timer.Reset(c.resubscribeDelay)
		}
	}
}

// checkChainID checks that the client is connected to the right L1 client
func (c *Client) checkChainID(ctx context.Context) error {
	const chainIDCheckTimeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, chainIDCheckTimeout)
	defer cancel()

	l1ChainID, err := c.settlement.ChainID(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf(
				"eth_chainId did not respond within %s; is --eth-node a valid Ethereum L1 RPC URL?",
				chainIDCheckTimeout,
			)
		}
		return fmt.Errorf("get Ethereum chain id: %w", err)
	}

	expectedL1ChainID := c.network.L1ChainID
	if l1ChainID.Cmp(expectedL1ChainID) == 0 {
		return nil
	}

	// NOTE: for now we return an error. If we want to support users who fork
	// Starknet to create a "custom" Starknet network, we will need to log a warning instead.
	return fmt.Errorf(
		"mismatched network id between L1 and L2. L2 network is %s; "+
			"is --eth-node pointing to the right network?",
		c.network.String(),
	)
}

func (c *Client) Run(ctx context.Context) error {
	defer c.settlement.Close()
	if err := c.checkChainID(ctx); err != nil {
		return err
	}

	// catchUpL1HeadUpdates is best-effort: a backward eth_getLogs scan can fail
	// on free-tier RPC providers that cap range size or rate-limit. On failure
	// we skip catch-up and proceed straight to the live subscription — the L1
	// head will lag until the next on-chain LogStateUpdate is observed, which
	// is acceptable rather terminating the execution.
	if err := c.catchUpL1HeadUpdates(ctx); err != nil {
		c.logger.Warn(
			"L1 head catch-up failed; resuming with live subscription only",
			zap.Error(err),
		)
	}

	return c.watchL1StateUpdates(ctx)
}

// CatchUpL1Head verifies the chain ID then writes the L1 head to the
// database, without entering the live subscription loop. Closes the
// underlying SettlementLayer on return; the Client must not be reused.
func (c *Client) CatchUpL1Head(ctx context.Context) error {
	defer c.settlement.Close()
	if err := c.checkChainID(ctx); err != nil {
		return err
	}
	return c.catchUpL1HeadUpdates(ctx)
}

func (c *Client) watchL1StateUpdates(ctx context.Context) error {
	buffer := 128

	c.logger.Info("Subscribing to L1 updates...")

	updateChan := make(chan *StateUpdate, buffer)
	updateSub := c.subscribeToUpdates(ctx, updateChan)
	if updateSub == nil {
		return nil
	}
	// Closure form so the deferred Unsubscribe targets the *current*
	// updateSub at function exit; reassigning updateSub during a
	// resubscribe would otherwise leak the new sub. A plain
	// `defer updateSub.Unsubscribe()` would also stack a new defer per
	// reconnect — unbounded growth on a long-running node.
	defer func() {
		if updateSub != nil {
			updateSub.Unsubscribe()
		}
	}()

	c.logger.Info("Subscribed to L1 updates")

	ticker := time.NewTicker(c.pollFinalisedInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		Outer:
			for {
				select {
				case err := <-updateSub.Err():
					// We can't use a warn log level here since we guarantee the L1 url will only be printed
					// in debug logs and panics (to avoid leaking the API key).
					c.logger.Debug("L1 update subscription failed, resubscribing", zap.Error(err))
					updateSub.Unsubscribe()

					updateSub = c.subscribeToUpdates(ctx, updateChan)
					if updateSub == nil {
						return nil
					}
				case update := <-updateChan:
					c.applyStateUpdate(update)
				default:
					break Outer
				}
			}

			if err := c.setL1Head(ctx); err != nil {
				return err
			}
		}
	}
}

// applyStateUpdate merges a StateUpdate (from either the forward
// subscription or the historical filter) into nonFinalisedLogs. A removed
// log clears all entries at or above its L1 block number.
func (c *Client) applyStateUpdate(u *StateUpdate) {
	c.logger.Debug(
		"Received L1 state update",
		zap.Uint64("l2Block", u.L2BlockNumber),
		zap.String("stateRoot", u.StateRoot.ShortString()),
		zap.String("l2BlockHash", u.L2BlockHash.ShortString()),
	)
	if u.Removed {
		for l1BlockNumber := range c.nonFinalisedLogs {
			if l1BlockNumber >= u.L1RefHeight {
				delete(c.nonFinalisedLogs, l1BlockNumber)
			}
		}
	} else {
		c.nonFinalisedLogs[u.L1RefHeight] = u
	}
}

// catchUpL1HeadUpdates performs a backward scan of historical LogStateUpdate
// events emitted while the node was offline (or before it ever ran), populating
// nonFinalisedLogs so that the first setL1Head call can write an L1 head
// without waiting for the next forward event. The scan walks back from
// LatestHeight in chunks of catchUpChunkSize until at least one finalised event
// is captured if any exists in the scanned range.
//
// On a mid-scan error the function returns without rolling back: entries
// already merged into nonFinalisedLogs are real on-chain events and remain
// usable by setL1Head and by the live subscription that runs afterward. The
// caller (Run) treats the error as best-effort and proceeds to the live
// subscription, so the partial state is an additive head-start, not a leak.
func (c *Client) catchUpL1HeadUpdates(ctx context.Context) error {
	const (
		heightCallTimeout = 30 * time.Second
		filterCallTimeout = 60 * time.Second
	)

	latestCtx, cancelLatest := context.WithTimeout(ctx, heightCallTimeout)
	latest, err := c.settlement.LatestHeight(latestCtx)
	cancelLatest()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf(
				"eth_blockNumber did not respond within %s; is --eth-node responsive?",
				heightCallTimeout,
			)
		}
		return fmt.Errorf("get latest L1 height: %w", err)
	}

	finalisedCtx, cancelFinalised := context.WithTimeout(ctx, heightCallTimeout)
	finalised, err := c.settlement.FinalisedHeight(finalisedCtx)
	cancelFinalised()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf(
				`eth_getBlockByNumber("finalized") did not respond within %s; `+
					`does --eth-node support the finalized block tag?`,
				heightCallTimeout,
			)
		}
		return fmt.Errorf("get finalised L1 height: %w", err)
	}

	c.logger.Info(
		"L1 catch-up starting",
		zap.Uint64("latest", latest),
		zap.Uint64("finalised", finalised),
		zap.Uint64("chunkSize", c.catchUpChunkSize),
	)

	var chunks, total int
	foundFinalised := false
	to := latest
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var from uint64
		if to+1 > c.catchUpChunkSize {
			from = to + 1 - c.catchUpChunkSize
		}
		filterCtx, cancelFilter := context.WithTimeout(ctx, filterCallTimeout)
		events, err := c.settlement.FilterStateUpdate(filterCtx, from, to)
		cancelFilter()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf(
					"eth_getLogs did not respond within %s; is --eth-node responsive?",
					filterCallTimeout,
				)
			}
			return err
		}
		chunks++
		total += len(events)
		for _, ev := range events {
			c.applyStateUpdate(ev)
			if ev.L1RefHeight <= finalised {
				foundFinalised = true
			}
		}
		// Stop once we've captured at least one finalised event (so setL1Head
		// has something to commit) or we've walked back to genesis.
		if foundFinalised || from == 0 {
			c.logger.Info(
				"L1 catch-up complete",
				zap.Int("chunks", chunks),
				zap.Int("events", total),
				zap.Int("nonFinalisedLogs", len(c.nonFinalisedLogs)),
				zap.Bool("foundFinalised", foundFinalised),
			)
			return c.setL1Head(ctx)
		}
		to = from - 1
	}
}

// finalisedHeight blocks until the L1 finalised height is retrieved. If
// context is cancelled, returns false
func (c *Client) finalisedHeight(ctx context.Context) (uint64, bool) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return 0, false
		case <-timer.C:
			const finalisedHeightTimeout = 30 * time.Second
			callCtx, cancel := context.WithTimeout(ctx, finalisedHeightTimeout)
			finalisedHeight, err := c.settlement.FinalisedHeight(callCtx)
			cancel()
			if err == nil {
				return finalisedHeight, true
			}
			c.logger.Debug("Failed to retrieve L1 finalised height, retrying...", zap.Error(err))
			timer.Reset(c.resubscribeDelay)
		}
	}
}

func (c *Client) setL1Head(ctx context.Context) error {
	finalisedHeight, found := c.finalisedHeight(ctx)
	if !found {
		return nil
	}

	// Get max finalised Starknet head.
	var maxFinalisedNumber uint64
	var maxFinalisedHead *StateUpdate
	for l1BlockNumber := range c.nonFinalisedLogs {
		if l1BlockNumber <= finalisedHeight {
			if l1BlockNumber >= maxFinalisedNumber {
				maxFinalisedNumber = l1BlockNumber
				maxFinalisedHead = c.nonFinalisedLogs[maxFinalisedNumber]
			}
			delete(c.nonFinalisedLogs, l1BlockNumber)
		}
	}

	// No finalised logs.
	if maxFinalisedHead == nil {
		return nil
	}

	head := &core.L1Head{
		BlockNumber: maxFinalisedHead.L2BlockNumber,
		BlockHash:   maxFinalisedHead.L2BlockHash,
		StateRoot:   maxFinalisedHead.StateRoot,
	}
	if err := c.l2Chain.SetL1Head(head); err != nil {
		return fmt.Errorf(
			"l1 head for block %d and state root %s: %w",
			head.BlockNumber, head.StateRoot.String(), err,
		)
	}
	c.listener.OnNewL1Head(head)
	c.logger.Info(
		"Updated l1 head",
		zap.Uint64("blockNumber", head.BlockNumber),
		zap.String("blockHash", head.BlockHash.ShortString()),
		zap.String("stateRoot", head.StateRoot.ShortString()),
	)

	return nil
}
