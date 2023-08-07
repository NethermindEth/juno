package l1

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/event"
)

type L1 interface {
	WatchL1Heads(ctx context.Context, sink chan<- *core.L1Head) event.Subscription
	Close()
}

type Client struct {
	eth     L1
	l2Chain *blockchain.Blockchain
	log     utils.SimpleLogger
	network utils.Network
}

var _ service.Service = (*Client)(nil)

func NewClient(eth L1, chain *blockchain.Blockchain, log utils.SimpleLogger) *Client {
	return &Client{
		eth:     eth,
		l2Chain: chain,
		log:     log,
		network: chain.Network(),
	}
}

func (c *Client) Run(ctx context.Context) error {
	defer c.eth.Close()
	buffer := 128
	l1HeadChan := make(chan *core.L1Head, buffer)
	sub := c.eth.WatchL1Heads(ctx, l1HeadChan)
	defer sub.Unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-sub.Err():
			return err
		case l1Head := <-l1HeadChan:
			if err := c.l2Chain.SetL1Head(l1Head); err != nil {
				return fmt.Errorf("store L1 head: %w", err)
			}
			c.log.Infow("Updated L1 head",
				"blockNumber", l1Head.BlockNumber,
				"blockHash", l1Head.BlockHash,
				"stateRoot", l1Head.StateRoot)
		}
	}
}
