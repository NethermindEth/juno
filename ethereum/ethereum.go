package ethereum

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/ethereum/contract"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

type Ethereum struct {
	network                utils.Network
	log                    utils.SimpleLogger
	checkFinalisedTicker   Ticker
	client                 *rpc.Client
	ethClient              *ethclient.Client
	filterer               *contract.StarknetFilterer
	maxResubscribeWaitTime time.Duration
	nonFinalisedLogs       map[uint64]*contract.StarknetLogStateUpdate
}

func DialWithContext(ctx context.Context, url string, network utils.Network, log utils.SimpleLogger) (*Ethereum, error) {
	// TODO replace with our own JSON-RPC client once we have one.
	// Geth pulls in a lot of dependencies that we don't use.
	client, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("dial Ethereum client: %w", err)
	}
	ethClient := ethclient.NewClient(client)

	coreAddress, err := network.CoreContractAddress()
	if err != nil {
		return nil, err
	}
	filterer, err := contract.NewStarknetFilterer(coreAddress, ethClient)
	if err != nil {
		return nil, fmt.Errorf("setup L1 log filterer: %w", err)
	}

	return &Ethereum{
		network:          network,
		log:              log,
		nonFinalisedLogs: make(map[uint64]*contract.StarknetLogStateUpdate, 0),
		checkFinalisedTicker: &ticker{
			t: time.NewTicker(time.Minute),
		},
		client:                 client,
		ethClient:              ethClient,
		filterer:               filterer,
		maxResubscribeWaitTime: time.Minute,
	}, nil
}

func (e *Ethereum) WithMaxResubscribeWaitTime(d time.Duration) *Ethereum {
	e.maxResubscribeWaitTime = d
	return e
}

func (e *Ethereum) WithCheckFinalisedTicker(t Ticker) *Ethereum {
	e.checkFinalisedTicker = t
	return e
}

type Ticker interface {
	Tick() <-chan time.Time
	Stop()
}

type ticker struct {
	t *time.Ticker
}

func (t *ticker) Tick() <-chan time.Time {
	return t.t.C
}

func (t *ticker) Stop() {
	t.t.Stop()
}

func (e *Ethereum) WatchL1Heads(ctx context.Context, sink chan<- *core.L1Head) event.Subscription {
	return event.NewSubscription(func(unsubscribe <-chan struct{}) error {
		if err := e.checkChainID(ctx); err != nil {
			return err
		}

		const buffer = 128
		updateChan := make(chan *contract.StarknetLogStateUpdate, buffer)
		firstTry := true // Only used for the resubscribe closure below.
		defer event.ResubscribeErr(e.maxResubscribeWaitTime, func(ctx context.Context, lastErr error) (event.Subscription, error) {
			if !firstTry {
				msg := "Subscription to Ethereum client failed, resubscribing..."
				// It is possible for the last error to be nil when the subscription attempt itself fails.
				// When the attempt succeeds and (usually much later) an error is received on the sub.Err() channel,
				// lastErr will not be nil.
				if lastErr == nil {
					e.log.Warnw(msg)
				} else {
					e.log.Warnw(msg, "err", lastErr)
				}
			}
			firstTry = false
			return e.filterer.WatchLogStateUpdate(&bind.WatchOpts{Context: ctx}, updateChan)
		}).Unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-unsubscribe:
				return nil
			case logStateUpdate := <-updateChan:
				e.log.Debugw("Received L1 LogStateUpdate",
					"number", logStateUpdate.BlockNumber,
					"stateRoot", logStateUpdate.GlobalRoot.Text(felt.Base16),
					"blockHash", logStateUpdate.BlockHash.Text(felt.Base16))
				if logStateUpdate.Raw.Removed {
					for l1BlockNumber := range e.nonFinalisedLogs {
						if l1BlockNumber >= logStateUpdate.Raw.BlockNumber {
							delete(e.nonFinalisedLogs, l1BlockNumber)
						}
					}
					// TODO What if the finalised block is also reorged?
				} else {
					e.nonFinalisedLogs[logStateUpdate.Raw.BlockNumber] = logStateUpdate
				}
			case <-e.checkFinalisedTicker.Tick():
				if logStateUpdate := e.latestHead(ctx); logStateUpdate != nil {
					sink <- &core.L1Head{
						BlockNumber: logStateUpdate.BlockNumber.Uint64(),
						BlockHash:   new(felt.Felt).SetBigInt(logStateUpdate.BlockHash),
						StateRoot:   new(felt.Felt).SetBigInt(logStateUpdate.GlobalRoot),
					}
				}
			}
		}
	})
}

func (e *Ethereum) Close() {
	e.client.Close()
	e.ethClient.Close()
}

func (e *Ethereum) checkChainID(ctx context.Context) error {
	gotChainID, err := e.ethClient.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("retrieve Ethereum chain ID: %w", err)
	}

	wantChainID := e.network.DefaultL1ChainID()
	if gotChainID.Cmp(wantChainID) == 0 {
		return nil
	}

	// NOTE: for now we return an error. If we want to support users who fork
	// Starknet to create a "custom" Starknet network, we will need to log a warning instead.
	return fmt.Errorf("mismatched L1 and L2 networks: L2 network %s; is the L1 node on the correct network?", e.network)
}

func (e *Ethereum) latestHead(ctx context.Context) *contract.StarknetLogStateUpdate {
	if len(e.nonFinalisedLogs) == 0 {
		return nil
	}

	var finalisedHeight uint64
FinalisedHeight:
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			var err error
			finalisedHeight, err = e.finalisedHeight(ctx)
			if err == nil {
				break FinalisedHeight
			}
			e.log.Warnw("Failed to retrieve L1 finalised height, retrying...", "error", err)
		}
	}

	// Get max finalised Starknet head.
	var maxFinalisedNumber uint64
	var maxFinalisedHead *contract.StarknetLogStateUpdate
	for l1BlockNumber := range e.nonFinalisedLogs {
		if l1BlockNumber <= finalisedHeight {
			if l1BlockNumber >= maxFinalisedNumber {
				maxFinalisedNumber = l1BlockNumber
				maxFinalisedHead = e.nonFinalisedLogs[maxFinalisedNumber]
			}
			delete(e.nonFinalisedLogs, l1BlockNumber)
		}
	}

	return maxFinalisedHead
}

func (e *Ethereum) finalisedHeight(ctx context.Context) (uint64, error) {
	finalisedBlock := make(map[string]any, 0)
	if err := e.client.CallContext(ctx, &finalisedBlock, "eth_getBlockByNumber", "finalized", false); err != nil { //nolint:misspell
		return 0, fmt.Errorf("get finalised Ethereum block: %w", err)
	}

	number, ok := finalisedBlock["number"]
	if !ok {
		return 0, fmt.Errorf("number field not present in Ethereum block")
	}

	numberString, ok := number.(string)
	if !ok {
		return 0, fmt.Errorf("block number is not a string: %v", number)
	}

	numberString = strings.TrimPrefix(numberString, "0x")
	numberUint, err := strconv.ParseUint(numberString, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parse block number: %s", numberString)
	}

	return numberUint, nil
}
