package l1

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/event"
)

//go:generate mockgen -destination=../mocks/mock_subscriber.go -package=mocks github.com/NethermindEth/juno/l1 Subscriber
type Subscriber interface {
	FinalisedHeight(ctx context.Context) (uint64, error)
	WatchLogStateUpdate(ctx context.Context, sink chan<- *contract.StarknetLogStateUpdate) (event.Subscription, error)
	ChainID(ctx context.Context) (*big.Int, error)

	Close()
}

type Client struct {
	l1                    Subscriber
	l2Chain               *blockchain.Blockchain
	log                   utils.SimpleLogger
	network               *utils.Network
	resubscribeDelay      time.Duration
	pollFinalisedInterval time.Duration
	nonFinalisedLogs      map[uint64]*contract.StarknetLogStateUpdate
	listener              EventListener
}

var _ service.Service = (*Client)(nil)

func NewClient(l1 Subscriber, chain *blockchain.Blockchain, log utils.SimpleLogger) *Client {
	return &Client{
		l1:                    l1,
		l2Chain:               chain,
		log:                   log,
		network:               chain.Network(),
		resubscribeDelay:      10 * time.Second,
		pollFinalisedInterval: time.Minute,
		nonFinalisedLogs:      make(map[uint64]*contract.StarknetLogStateUpdate, 0),
		listener:              SelectiveListener{},
	}
}

func (c *Client) WithEventListener(l EventListener) *Client {
	c.listener = l
	return c
}

func (c *Client) WithResubscribeDelay(delay time.Duration) *Client {
	c.resubscribeDelay = delay
	return c
}

// WithPollFinalisedInterval sets the time to wait before checking for an update to the finalised L1 block.
func (c *Client) WithPollFinalisedInterval(delay time.Duration) *Client {
	c.pollFinalisedInterval = delay
	return c
}

func (c *Client) subscribeToUpdates(ctx context.Context, updateChan chan *contract.StarknetLogStateUpdate) (event.Subscription, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled before resubscribe was successful: %w", ctx.Err())
		default:
			updateSub, err := c.l1.WatchLogStateUpdate(ctx, updateChan)
			if err == nil {
				return updateSub, nil
			}
			c.log.Debugw("Failed to subscribe to L1 state updates", "tryAgainIn", c.resubscribeDelay, "err", err)
			time.Sleep(c.resubscribeDelay)
		}
	}
}

func (c *Client) checkChainID(ctx context.Context) error {
	gotChainID, err := c.l1.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("retrieve Ethereum chain ID: %w", err)
	}

	wantChainID := c.network.L1ChainID
	if gotChainID.Cmp(wantChainID) == 0 {
		return nil
	}

	// NOTE: for now we return an error. If we want to support users who fork
	// Starknet to create a "custom" Starknet network, we will need to log a warning instead.
	return fmt.Errorf("mismatched L1 and L2 networks: L2 network %s; is the L1 node on the correct network?", c.network.String())
}

func (c *Client) Run(ctx context.Context) error {
	defer c.l1.Close()
	if err := c.checkChainID(ctx); err != nil {
		return err
	}

	buffer := 128

	c.log.Infow("Subscribing to L1 updates...")

	updateChan := make(chan *contract.StarknetLogStateUpdate, buffer)
	updateSub, err := c.subscribeToUpdates(ctx, updateChan)
	if err != nil {
		return err
	}
	defer updateSub.Unsubscribe()

	c.log.Infow("Subscribed to L1 updates")

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
					c.log.Debugw("L1 update subscription failed, resubscribing", "error", err)
					updateSub.Unsubscribe()

					updateSub, err = c.subscribeToUpdates(ctx, updateChan)
					if err != nil {
						return err
					}
					defer updateSub.Unsubscribe() //nolint:gocritic
				case logStateUpdate := <-updateChan:
					c.log.Debugw("Received L1 LogStateUpdate",
						"number", logStateUpdate.BlockNumber,
						"stateRoot", logStateUpdate.GlobalRoot.Text(felt.Base16),
						"blockHash", logStateUpdate.BlockHash.Text(felt.Base16))

					if logStateUpdate.Raw.Removed {
						for l1BlockNumber := range c.nonFinalisedLogs {
							if l1BlockNumber >= logStateUpdate.Raw.BlockNumber {
								delete(c.nonFinalisedLogs, l1BlockNumber)
							}
						}
						// TODO What if the finalised block is also reorged?
					} else {
						c.nonFinalisedLogs[logStateUpdate.Raw.BlockNumber] = logStateUpdate
					}
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

func (c *Client) finalisedHeight(ctx context.Context) uint64 {
	for {
		select {
		case <-ctx.Done():
			return 0
		default:
			finalisedHeight, err := c.l1.FinalisedHeight(ctx)
			if err == nil {
				return finalisedHeight
			}
			c.log.Debugw("Failed to retrieve L1 finalised height, retrying...", "error", err)
		}
	}
}

func (c *Client) setL1Head(ctx context.Context) error {
	finalisedHeight := c.finalisedHeight(ctx)

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
		return fmt.Errorf("l1 head for block %d and state root %s: %w", head.BlockNumber, head.StateRoot.String(), err)
	}
	c.listener.OnNewL1Head(head)
	c.log.Infow("Updated l1 head",
		"blockNumber", head.BlockNumber,
		"blockHash", head.BlockHash.ShortString(),
		"stateRoot", head.StateRoot.ShortString())

	return nil
}
