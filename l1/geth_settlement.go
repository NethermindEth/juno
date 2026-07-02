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

// watchForwarderBuffer is the per-subscription buffer between the
// contract decoder and the l1.StateUpdate sink consumed by l1.Client.
const watchForwarderBuffer = 64

// gethFinalizedBlockNumber is the geth tag value for the latest
// finalised block; used as the "block number" arg to HeaderByNumber.
var gethFinalizedBlockNumber = new(big.Int).SetInt64(rpc.FinalizedBlockNumber.Int64())

// GethSettlement is the go-ethereum-backed SettlementLayer, wrapping
// ethclient plus the abigen StarknetFilterer to talk to the Starknet
// core L1 bridge. It also satisfies rpccore.L1Client (via
// TransactionReceipt), so node.go hands one instance to both the L1
// sync loop and the RPC handlers.
//
// WS connection drops are recovered below this layer: rpc.Client
// reconnects unary calls, and subscription drops surface on Err() for
// l1.Client.watchL1StateUpdates to resubscribe.
type GethSettlement struct {
	contractAddress eth.Address
	url             string

	rpcClient *rpc.Client
	ethClient *ethclient.Client
	filterer  *contract.StarknetFilterer

	listener EventListener
}

// NewGethSettlement dials the Ethereum endpoint at url and returns a
// ready-to-use settlement-layer adapter bound to contractAddress (the
// Starknet core L1 bridge). The transport is selected by URL scheme;
// ws/wss is required so log subscriptions work.
func NewGethSettlement(
	ctx context.Context,
	rawURL string,
	contractAddress eth.Address,
	opts ...GethSettlementOption,
) (*GethSettlement, error) {
	o := gethSettlementOptions{listener: SelectiveListener{}}
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

	return &GethSettlement{
		contractAddress: contractAddress,
		url:             rawURL,
		rpcClient:       rpcClient,
		ethClient:       ethClient,
		filterer:        filterer,
		listener:        o.listener,
	}, nil
}

// GethSettlementOption configures a GethSettlement at construction time.
type GethSettlementOption func(*gethSettlementOptions)

type gethSettlementOptions struct {
	listener EventListener
}

// WithSettlementListener sets the EventListener that fires OnL1Call on
// every Ethereum RPC method (default: no-op). Set at construction: the
// listener field is read unlocked from every RPC method, so it must not
// be mutated once the settlement is shared with a goroutine.
func WithSettlementListener(l EventListener) GethSettlementOption {
	return func(o *gethSettlementOptions) { o.listener = l }
}

// observe reports OnL1Call latency on return, on both success and
// failure paths.
func (s *GethSettlement) observe(method string) func() {
	t := time.Now()
	return func() { s.listener.OnL1Call(method, time.Since(t)) }
}

// ChainID returns the Ethereum chain id (eth_chainId).
func (s *GethSettlement) ChainID(ctx context.Context) (*big.Int, error) {
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
func (s *GethSettlement) FinalisedHeight(ctx context.Context) (uint64, error) {
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
func (s *GethSettlement) LatestHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_blockNumber")()
	n, err := s.ethClient.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("getting latest Ethereum block number: %w", err)
	}
	return n, nil
}

// FilterStateUpdate decodes every LogStateUpdate in [from, to] into
// the StateUpdate shape.
func (s *GethSettlement) FilterStateUpdate(
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
// already applied) on sink. Requires a ws/wss endpoint.
//
// Caller contract: sink MUST be drained promptly. A sink that stalls
// back-pressures the abigen subscription channel and eventually the
// underlying ws connection.
func (s *GethSettlement) WatchStateUpdate(
	ctx context.Context,
	sink chan<- *StateUpdate,
) (Subscription, error) {
	raw := make(chan *contract.StarknetLogStateUpdate, watchForwarderBuffer)
	inner, err := s.filterer.WatchLogStateUpdate(&bind.WatchOpts{Context: ctx}, raw)
	if err != nil {
		return nil, fmt.Errorf("subscribing to LogStateUpdate: %w", err)
	}
	return forwardStateUpdates(inner, raw, sink), nil
}

// TransactionReceipt fetches an L1 transaction receipt by hash. Used
// by the RPC handlers for starknet_getMessageStatus.
func (s *GethSettlement) TransactionReceipt(
	ctx context.Context,
	txHash eth.Hash,
) (*eth.Receipt, error) {
	defer s.observe("eth_getTransactionReceipt")()
	r, err := s.ethClient.TransactionReceipt(ctx, common.Hash(txHash))
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return nil, fmt.Errorf("getting transaction receipt: %w", eth.ErrNotFound)
		}
		return nil, fmt.Errorf("getting transaction receipt: %w", err)
	}
	return gethReceiptToEth(r), nil
}

// Close releases the underlying RPC client.
func (s *GethSettlement) Close() {
	s.ethClient.Close()
}

// stateUpdateFromGethContract translates the abigen-decoded event into
// the StateUpdate. felt conversion happens here so l1.Client never
// touches go-ethereum types.
func stateUpdateFromGethContract(ev *contract.StarknetLogStateUpdate) *StateUpdate {
	return &StateUpdate{
		L2BlockNumber: ev.BlockNumber.Uint64(),
		L2BlockHash:   new(felt.Felt).SetBigInt(ev.BlockHash),
		StateRoot:     new(felt.Felt).SetBigInt(ev.GlobalRoot),
		L1RefHeight:   ev.Raw.BlockNumber,
		Removed:       ev.Raw.Removed,
	}
}

// gethReceiptToEth copies the receipt fields juno reads. Only Logs is
// consumed today; a consumer needing other fields must extend this.
func gethReceiptToEth(r *types.Receipt) *eth.Receipt {
	logs := make([]eth.Log, len(r.Logs))
	for i, l := range r.Logs {
		logs[i] = gethLogToEth(l)
	}
	return &eth.Receipt{Logs: logs}
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

// forwardStateUpdates returns a subscription that decodes each raw event
// into a StateUpdate and forwards it to sink, until inner errors or the
// caller unsubscribes. Lifecycle (close, Unsubscribe, error delivery,
// teardown) is owned by event.NewSubscription; this adds only the type
// translation and the stall-vs-quit select.
func forwardStateUpdates(
	inner event.Subscription,
	raw <-chan *contract.StarknetLogStateUpdate,
	sink chan<- *StateUpdate,
) Subscription {
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer inner.Unsubscribe()
		for {
			select {
			case ev := <-raw:
				select {
				case sink <- stateUpdateFromGethContract(ev):
				case <-quit:
					return nil
				}
			case err := <-inner.Err():
				return err
			case <-quit:
				return nil
			}
		}
	})
}

// GethSettlement must satisfy both interfaces it serves.
var (
	_ SettlementLayer  = (*GethSettlement)(nil)
	_ rpccore.L1Client = (*GethSettlement)(nil)
)
