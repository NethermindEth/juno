package l1

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"go.uber.org/zap"
)

//go:generate mockgen -destination=../mocks/mock_subscriber.go -package=mocks github.com/NethermindEth/juno/l1 Subscriber
type Subscriber interface {
	FinalisedHeight(ctx context.Context) (uint64, error)
	LatestHeight(ctx context.Context) (uint64, error)
	WatchLogStateUpdate(ctx context.Context, sink chan<- *contract.StarknetLogStateUpdate) (event.Subscription, error)
	FilterLogStateUpdate(
		ctx context.Context,
		fromBlock,
		toBlock uint64,
	) ([]*contract.StarknetLogStateUpdate, error)
	ChainID(ctx context.Context) (*big.Int, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	Close()
}

type Client struct {
	l1                    Subscriber
	l2Chain               *blockchain.Blockchain
	logger                log.StructuredLogger
	network               *networks.Network
	resubscribeDelay      time.Duration
	pollFinalisedInterval time.Duration
	catchUpChunkSize      uint64
	nonFinalisedLogs      map[uint64]*contract.StarknetLogStateUpdate
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
	l1 Subscriber,
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
		l1:                    l1,
		l2Chain:               chain,
		logger:                logger,
		network:               chain.Network(),
		resubscribeDelay:      o.ResubscribeDelay,
		pollFinalisedInterval: o.PollFinalisedInterval,
		catchUpChunkSize:      o.CatchUpChunkSize,
		nonFinalisedLogs:      make(map[uint64]*contract.StarknetLogStateUpdate),
		listener:              o.EventListener,
	}
}

// subscribeToUpdates blocks until a subscription is established. If context is cancelled,
// returns nil
func (c *Client) subscribeToUpdates(
	ctx context.Context, updateChan chan *contract.StarknetLogStateUpdate,
) event.Subscription {
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
			updateSub, err := c.l1.WatchLogStateUpdate(ctx, updateChan)
			if err == nil {
				return updateSub
			}
			c.logger.Debug("Failed to subscribe to L1 state updates",
				zap.Duration("tryAgainIn", c.resubscribeDelay),
				zap.Error(err),
			)
			timer.Reset(c.resubscribeDelay)
		}
	}
}

// errChainIDMismatch marks an L1/L2 network mismatch: a misconfiguration that
// retrying cannot fix, so ensureChainID treats it as fatal. (Supporting custom
// forked Starknet networks would mean warning here instead of erroring.)
var errChainIDMismatch = errors.New("mismatched network id between L1 and L2")

// ensureChainID checks the L1 node is on the expected network, retrying transient
// failures (rate limits, timeouts, an unresponsive node) until the check passes,
// ctx is cancelled, or a network mismatch is found, so a flaky L1 never shuts the
// node down (issue #1385). A mismatch is a misconfiguration and is returned fatally.
//
// The timer is reset after each failed probe, so a full resubscribeDelay always
// elapses between the end of one attempt and the start of the next (gap is the
// probe's response time + resubscribeDelay) — never back-to-back requests.
func (c *Client) ensureChainID(ctx context.Context) error {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			switch err := c.checkChainID(ctx); {
			case err == nil:
				return nil
			case errors.Is(err, errChainIDMismatch):
				return err
			case ctx.Err() != nil:
				// Cancelled mid-probe: return the real cause, don't warn.
				return ctx.Err()
			default:
				c.logger.Warn("Failed to verify L1 chain ID, retrying",
					zap.Duration("tryAgainIn", c.resubscribeDelay),
					zap.Error(err),
				)
				timer.Reset(c.resubscribeDelay)
			}
		}
	}
}

// checkChainID runs a single chain-ID verification attempt (no retry); the
// one-shot CatchUpL1Head migration path relies on it failing fast.
func (c *Client) checkChainID(ctx context.Context) error {
	const chainIDCheckTimeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, chainIDCheckTimeout)
	defer cancel()

	l1ChainID, err := c.l1.ChainID(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf(
				"eth_chainId did not respond within %s; is --eth-node a valid Ethereum L1 RPC URL?",
				chainIDCheckTimeout,
			)
		}
		return fmt.Errorf("retrieving Ethereum chain ID: %w", err)
	}

	expectedL1ChainID := c.network.L1ChainID
	if l1ChainID.Cmp(expectedL1ChainID) == 0 {
		return nil
	}

	return fmt.Errorf(
		"%w. L2 network is %s; is --eth-node pointing to the right network?",
		errChainIDMismatch, c.network.String(),
	)
}

func (c *Client) Run(ctx context.Context) error {
	defer c.l1.Close()
	if err := c.ensureChainID(ctx); err != nil {
		// A cancelled context means we're shutting down, not a real failure;
		// a mismatch (the only other non-nil return) is fatal.
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	// catchUpL1HeadUpdates is best-effort: a backward eth_getLogs scan can fail
	// on free-tier RPC providers that cap range size or rate-limit. On failure
	// we skip catch-up and proceed straight to the live subscription — the L1
	// head will lag until the next on-chain LogStateUpdate is observed, which
	// is acceptable rather terminating the execution.
	if err := c.catchUpL1HeadUpdates(ctx); err != nil {
		c.logger.Warn("L1 head catch-up failed; resuming with live subscription only",
			zap.Error(err),
		)
	}

	return c.watchL1StateUpdates(ctx)
}

// CatchUpL1Head verifies the chain ID then writes the L1 head to the
// database, without entering the live subscription loop. Closes the
// underlying Subscriber on return; the Client must not be reused.
func (c *Client) CatchUpL1Head(ctx context.Context) error {
	defer c.l1.Close()
	if err := c.checkChainID(ctx); err != nil {
		return err
	}
	return c.catchUpL1HeadUpdates(ctx)
}

func (c *Client) watchL1StateUpdates(ctx context.Context) error {
	buffer := 128

	c.logger.Info("Subscribing to L1 updates...")

	updateChan := make(chan *contract.StarknetLogStateUpdate, buffer)
	updateSub := c.subscribeToUpdates(ctx, updateChan)
	if updateSub == nil {
		return nil
	}
	defer updateSub.Unsubscribe()

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
					// TODO can we use geth's event.Resubscribe?
					// We can't use a warn log level here since we guarantee the L1 url will only be printed
					// in debug logs and panics (to avoid leaking the API key).
					c.logger.Debug("L1 update subscription failed, resubscribing", zap.Error(err))
					updateSub.Unsubscribe()

					updateSub = c.subscribeToUpdates(ctx, updateChan)
					if updateSub == nil {
						return nil
					}
					defer updateSub.Unsubscribe() //nolint:gocritic
				case logStateUpdate := <-updateChan:
					c.applyLogStateUpdate(logStateUpdate)
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

// applyLogStateUpdate merges a LogStateUpdate (from either the forward
// subscription or the historical filter) into nonFinalisedLogs. A removed
// log clears all entries at or above its L1 block number.
func (c *Client) applyLogStateUpdate(u *contract.StarknetLogStateUpdate) {
	c.logger.Debug("Received L1 LogStateUpdate",
		zap.String("number", u.BlockNumber.String()),
		zap.String("stateRoot", u.GlobalRoot.Text(felt.Base16)),
		zap.String("blockHash", u.BlockHash.Text(felt.Base16)),
	)
	if u.Raw.Removed {
		for l1BlockNumber := range c.nonFinalisedLogs {
			if l1BlockNumber >= u.Raw.BlockNumber {
				delete(c.nonFinalisedLogs, l1BlockNumber)
			}
		}
	} else {
		c.nonFinalisedLogs[u.Raw.BlockNumber] = u
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
	latest, err := c.l1.LatestHeight(latestCtx)
	cancelLatest()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf(
				"eth_blockNumber did not respond within %s; is --eth-node responsive?",
				heightCallTimeout,
			)
		}
		return fmt.Errorf("failed to get latest height: %w", err)
	}

	finalisedCtx, cancelFinalised := context.WithTimeout(ctx, heightCallTimeout)
	finalised, err := c.l1.FinalisedHeight(finalisedCtx)
	cancelFinalised()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf(
				`eth_getBlockByNumber("finalized") did not respond within %s; `+
					`does --eth-node support the finalized block tag?`,
				heightCallTimeout,
			)
		}
		return fmt.Errorf("failed to get finalised height: %w", err)
	}

	c.logger.Info("L1 catch-up starting",
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
		events, err := c.l1.FilterLogStateUpdate(filterCtx, from, to)
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
			c.applyLogStateUpdate(ev)
			if ev.Raw.BlockNumber <= finalised {
				foundFinalised = true
			}
		}
		// Stop once we've captured at least one finalised event (so setL1Head
		// has something to commit) or we've walked back to genesis.
		if foundFinalised || from == 0 {
			c.logger.Info("L1 catch-up complete",
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
			finalisedHeight, err := c.l1.FinalisedHeight(callCtx)
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
	var maxFinalisedHead *contract.StarknetLogStateUpdate
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
		BlockNumber: maxFinalisedHead.BlockNumber.Uint64(),
		BlockHash:   new(felt.Felt).SetBigInt(maxFinalisedHead.BlockHash),
		StateRoot:   new(felt.Felt).SetBigInt(maxFinalisedHead.GlobalRoot),
	}
	if err := c.l2Chain.SetL1Head(head); err != nil {
		return fmt.Errorf(
			"l1 head for block %d and state root %s: %w",
			head.BlockNumber, head.StateRoot.String(), err,
		)
	}
	c.listener.OnNewL1Head(head)
	c.logger.Info("Updated l1 head",
		zap.Uint64("blockNumber", head.BlockNumber),
		zap.String("blockHash", head.BlockHash.ShortString()),
		zap.String("stateRoot", head.StateRoot.ShortString()),
	)

	return nil
}

func (c *Client) L1() Subscriber {
	return c.l1
}
