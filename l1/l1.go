package l1

import (
	"context"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/contract"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

//go:generate mockgen -destination=./mocks/mock_subscriber.go -package=mocks github.com/NethermindEth/juno/l1 Subscriber
type Subscriber interface {
	WatchHeader(ctx context.Context, sink chan<- *types.Header) (event.Subscription, error)
	WatchLogStateUpdate(ctx context.Context, sink chan<- *contract.StarknetLogStateUpdate) (event.Subscription, error)
	ChainID(ctx context.Context) (*big.Int, error)
}

type Client struct {
	l1                Subscriber
	l2Chain           *blockchain.Blockchain
	log               utils.SimpleLogger
	confirmationQueue *queue
	network           utils.Network
}

var _ service.Service = (*Client)(nil)

func NewClient(l1 Subscriber, chain *blockchain.Blockchain, confirmationPeriod uint64, log utils.SimpleLogger) *Client {
	return &Client{
		l1:                l1,
		l2Chain:           chain,
		log:               log,
		network:           chain.Network(),
		confirmationQueue: newQueue(confirmationPeriod),
	}
}

func (c *Client) subscribeToHeaders(ctx context.Context, headerChan chan *types.Header) (event.Subscription, error) {
	sub, err := c.l1.WatchHeader(ctx, headerChan)
	if err != nil {
		return nil, fmt.Errorf("subscribe to L1 headers: %w", err)
	}
	return sub, nil
}

func (c *Client) subscribeToUpdates(ctx context.Context,
	logStateUpdateChan chan *contract.StarknetLogStateUpdate,
) (event.Subscription, error) {
	sub, err := c.l1.WatchLogStateUpdate(ctx, logStateUpdateChan)
	if err != nil {
		return nil, fmt.Errorf("subscribe to L1 state updates: %w", err)
	}
	return sub, nil
}

func (c *Client) checkChainID(ctx context.Context) error {
	gotChainID, err := c.l1.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("retrieve Ethereum chain ID: %w", err)
	}

	wantChainID := c.network.DefaultL1ChainID()
	if gotChainID.Cmp(wantChainID) == 0 {
		return nil
	}

	// NOTE: for now we return an error. If we want to support users who fork
	// Starknet to create a "custom" Starknet network, we will need to log a warning instead.
	return fmt.Errorf("mismatched L1 and L2 networks: L2 network %s; is the L1 node on the correct network?", c.network)
}

func (c *Client) Run(ctx context.Context) error {
	if err := c.checkChainID(ctx); err != nil {
		return err
	}

	buffer := 128

	logStateUpdateChan := make(chan *contract.StarknetLogStateUpdate, buffer)
	subUpdates, err := c.subscribeToUpdates(ctx, logStateUpdateChan)
	if err != nil {
		return err
	}
	defer subUpdates.Unsubscribe()

	headerChan := make(chan *types.Header, buffer)
	subHeaders, err := c.subscribeToHeaders(ctx, headerChan)
	if err != nil {
		return err
	}
	defer subHeaders.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-subHeaders.Err():
			c.log.Warnw("L1 header subscription failed, resubscribing", "error", err)
			subHeaders.Unsubscribe()

			subHeaders, err = c.subscribeToHeaders(ctx, headerChan)
			if err != nil {
				return err
			}
			defer subHeaders.Unsubscribe() //nolint:gocritic
		case header := <-headerChan:
			l1Height := header.Number.Uint64()
			c.log.Debugw("Received L1 header", "number", l1Height, "hash", header.Hash().Hex())
		Outer:
			// Check for updates.
			// We need to loop in case there were multiple LogStateUpdates emitted.
			for {
				select {
				case err := <-subUpdates.Err():
					c.log.Warnw("L1 update subscription failed, resubscribing", "error", err)
					subUpdates.Unsubscribe()

					subUpdates, err = c.subscribeToUpdates(ctx, logStateUpdateChan)
					if err != nil {
						return err
					}
					defer subUpdates.Unsubscribe() //nolint:gocritic
				case logStateUpdate := <-logStateUpdateChan:
					c.log.Debugw("Received L1 LogStateUpdate",
						"number", logStateUpdate.BlockNumber,
						"stateRoot", logStateUpdate.GlobalRoot.Text(felt.Base16),
						"blockHash", logStateUpdate.BlockHash.Text(felt.Base16))
					if logStateUpdate.Raw.Removed {
						// NOTE: we only modify the local confirmationQueue upon receiving reorged logs.
						// We assume new logs will soon follow, so we don't notify the l2Chain.
						c.confirmationQueue.Reorg(logStateUpdate.Raw.BlockNumber)
					} else {
						c.confirmationQueue.Enqueue(logStateUpdate)
					}
				default:
					break Outer
				}
			}

			if err := c.setL1Height(l1Height); err != nil {
				return err
			}
		}
	}
}

func (c *Client) setL1Height(l1Height uint64) error {
	// Set the chain head to the max confirmed log, if it exists.
	if maxConfirmed := c.confirmationQueue.MaxConfirmed(l1Height); maxConfirmed != nil {
		head := &core.L1Head{
			BlockNumber: maxConfirmed.BlockNumber.Uint64(),
			BlockHash:   new(felt.Felt).SetBigInt(maxConfirmed.BlockHash),
			StateRoot:   new(felt.Felt).SetBigInt(maxConfirmed.GlobalRoot),
		}
		if err := c.l2Chain.SetL1Head(head); err != nil {
			return fmt.Errorf("l1 head for block %d and state root %s: %w", head.BlockNumber, head.StateRoot.String(), err)
		}
		c.confirmationQueue.Remove(maxConfirmed.Raw.BlockNumber)
		c.log.Infow("Updated l1 head",
			"blockNumber", head.BlockNumber,
			"blockHash", head.BlockHash.ShortString(),
			"stateRoot", head.StateRoot.ShortString())
	}
	return nil
}
