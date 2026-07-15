package l1

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/geth/contract"
	"github.com/NethermindEth/juno/rpc/rpccore"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

// GethL1StateProvider must satisfy both interfaces it serves.
var (
	_ L1StateProvider  = (*GethL1StateProvider)(nil)
	_ rpccore.L1Client = (*GethL1StateProvider)(nil)
)

// watchForwarderBuffer is the per-subscription buffer between the
// contract decoder and the l1.StateUpdate sink consumed by l1.Client.
const watchForwarderBuffer = 64

// gethFinalizedBlockNumber is the geth tag value for the latest
// finalised block; used as the "block number" arg to HeaderByNumber.
var gethFinalizedBlockNumber = new(big.Int).SetInt64(rpc.FinalizedBlockNumber.Int64())

// GethL1StateProvider is the go-ethereum-backed L1StateProvider, wrapping
// ethclient plus the abigen StarknetFilterer to talk to the Starknet
// core L1 bridge. It also satisfies rpccore.L1Client (via
// TransactionReceipt), so node.go hands one instance to both the L1
// sync loop and the RPC handlers.
//
// WS connection drops are recovered below this layer: rpc.Client
// reconnects unary calls, and subscription drops surface on Err() for
// l1.Client.watchL1StateUpdates to resubscribe.
type GethL1StateProvider struct {
	ethClient *ethclient.Client
	filterer  *contract.StarknetFilterer
	listener  EventListener
}

// NewGethL1StateProvider dials the Ethereum endpoint at url and returns a
// ready-to-use L1StateProvider bound to contractAddress (the
// Starknet core L1 bridge). The transport is selected by URL scheme;
// ws/wss is required so log subscriptions work.
func NewGethL1StateProvider(
	ctx context.Context,
	rawURL string,
	contractAddress eth.Address,
	opts ...GethL1StateProviderOption,
) (*GethL1StateProvider, error) {
	o := gethL1StateProviderOptions{listener: SelectiveListener{}}
	for _, opt := range opts {
		opt(&o)
	}

	rpcClient, err := rpc.DialContext(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("dialing L1: %w", err)
	}
	ethClient := ethclient.NewClient(rpcClient)
	filterer, err := contract.NewStarknetFilterer(common.Address(contractAddress), ethClient)
	if err != nil {
		ethClient.Close()
		return nil, fmt.Errorf("binding Starknet filterer: %w", err)
	}

	return &GethL1StateProvider{
		ethClient: ethClient,
		filterer:  filterer,
		listener:  o.listener,
	}, nil
}

// GethL1StateProviderOption configures a GethL1StateProvider at construction time.
type GethL1StateProviderOption func(*gethL1StateProviderOptions)

type gethL1StateProviderOptions struct {
	listener EventListener
}

func WithL1StateProviderListener(l EventListener) GethL1StateProviderOption {
	return func(o *gethL1StateProviderOptions) { o.listener = l }
}

// observe reports OnL1Call latency on return, on both success and
// failure paths.
func (s *GethL1StateProvider) observe(method string) func() {
	t := time.Now()
	return func() { s.listener.OnL1Call(method, time.Since(t)) }
}

// ChainID returns the Ethereum chain id (eth_chainId).
func (s *GethL1StateProvider) ChainID(ctx context.Context) (*big.Int, error) {
	defer s.observe("eth_chainId")()
	id, err := s.ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting chain id: %w", err)
	}
	return id, nil
}

// FinalisedHeight returns the latest finalised L1 block number. A
// missing finalised header is reported as eth.ErrNotFound so callers
// can distinguish "node hasn't seen finality yet" from a transport
// failure.
func (s *GethL1StateProvider) FinalisedHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_getBlockByNumber")()
	head, err := s.ethClient.HeaderByNumber(ctx, gethFinalizedBlockNumber)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return 0, fmt.Errorf("finalised block not found: %w", eth.ErrNotFound)
		}
		return 0, fmt.Errorf("getting finalised Ethereum block: %w", err)
	}
	return head.Number.Uint64(), nil
}

// LatestHeight returns the latest known L1 block number (eth_blockNumber).
func (s *GethL1StateProvider) LatestHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_blockNumber")()
	n, err := s.ethClient.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("getting latest Ethereum block number: %w", err)
	}
	return n, nil
}

// FilterStateUpdate decodes every LogStateUpdate in [from, to] into
// the StateUpdate shape.
func (s *GethL1StateProvider) FilterStateUpdate(
	ctx context.Context,
	from, to uint64,
) ([]*StateUpdate, error) {
	defer s.observe("eth_getLogs")()
	events, err := s.filterer.FilterLogStateUpdate(&bind.FilterOpts{
		Context: ctx,
		Start:   from,
		End:     &to,
	})
	if err != nil {
		return nil, fmt.Errorf("filtering LogStateUpdate [%d,%d]: %w", from, to, err)
	}
	out := make([]*StateUpdate, len(events))
	for i, ev := range events {
		out[i] = stateUpdateFromGethContract(ev)
	}
	return out, nil
}

// WatchStateUpdate subscribes to live LogStateUpdate events and
// forwards each one (decoded into StateUpdate, with felt conversion
// already applied) on updatesCh. Requires a ws/wss endpoint.
//
// Caller contract: updatesCh MUST be drained promptly. A channel that
// stalls back-pressures the abigen subscription channel and eventually
// the underlying ws connection.
func (s *GethL1StateProvider) WatchStateUpdate(
	ctx context.Context,
	updatesCh chan<- *StateUpdate,
) (Subscription, error) {
	gethEventsCh := make(chan *contract.StarknetLogStateUpdate, watchForwarderBuffer)
	gethSub, err := s.filterer.WatchLogStateUpdate(&bind.WatchOpts{Context: ctx}, gethEventsCh)
	if err != nil {
		return nil, fmt.Errorf("subscribing to LogStateUpdate: %w", err)
	}
	return forwardStateUpdates(gethSub, gethEventsCh, updatesCh), nil
}

// TransactionReceipt fetches an L1 transaction receipt by hash. Used
// by the RPC handlers for starknet_getMessageStatus.
func (s *GethL1StateProvider) TransactionReceipt(
	ctx context.Context,
	txHash eth.Hash,
) (eth.Receipt, error) {
	defer s.observe("eth_getTransactionReceipt")()
	r, err := s.ethClient.TransactionReceipt(ctx, common.Hash(txHash))
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return eth.Receipt{}, fmt.Errorf("getting transaction receipt: %w", eth.ErrNotFound)
		}
		return eth.Receipt{}, fmt.Errorf("getting transaction receipt: %w", err)
	}
	return gethReceiptToEth(r), nil
}

// Close releases the underlying RPC client.
func (s *GethL1StateProvider) Close() {
	s.ethClient.Close()
}

func stateUpdateFromGethContract(ev *contract.StarknetLogStateUpdate) *StateUpdate {
	var blockHash, stateRoot felt.Felt
	blockHash.SetBigInt(ev.BlockHash)
	stateRoot.SetBigInt(ev.GlobalRoot)
	return &StateUpdate{
		L2BlockNumber: ev.BlockNumber.Uint64(),
		L2BlockHash:   blockHash,
		StateRoot:     stateRoot,
		L1RefHeight:   ev.Raw.BlockNumber,
		Removed:       ev.Raw.Removed,
	}
}

// gethReceiptToEth copies the receipt fields juno reads. Only Logs is
// consumed today; a consumer needing other fields must extend this.
func gethReceiptToEth(r *types.Receipt) eth.Receipt {
	logs := make([]eth.Log, len(r.Logs))
	for i, l := range r.Logs {
		logs[i] = gethLogToEth(l)
	}
	return eth.Receipt{Logs: logs}
}

func gethLogToEth(l *types.Log) eth.Log {
	topics := make([]eth.Hash, len(l.Topics))
	for i, t := range l.Topics {
		topics[i] = eth.Hash(t)
	}
	return eth.Log{
		Topics:      topics,
		Data:        eth.DataBytes(l.Data),
		BlockNumber: eth.HexU64(l.BlockNumber),
		Removed:     l.Removed,
	}
}

// forwardStateUpdates returns a subscription that decodes each geth event
// into a StateUpdate and forwards it to updatesCh, until gethSub errors or
// the caller unsubscribes. Lifecycle (close, Unsubscribe, error delivery,
// teardown) is owned by event.NewSubscription; this adds only the type
// translation and the stall-vs-quit select.
func forwardStateUpdates(
	gethSub event.Subscription,
	gethEventsCh <-chan *contract.StarknetLogStateUpdate,
	updatesCh chan<- *StateUpdate,
) Subscription {
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer gethSub.Unsubscribe()
		for {
			select {
			case ev := <-gethEventsCh:
				select {
				case updatesCh <- stateUpdateFromGethContract(ev):
				case <-quit:
					return nil
				}
			case err := <-gethSub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	})
}
