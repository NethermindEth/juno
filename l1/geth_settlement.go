package l1

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/geth/contract"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils/log"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

// gethFinalizedBlockNumber is the geth tag value for the latest
// finalised block; used as the "block number" arg to HeaderByNumber.
var gethFinalizedBlockNumber = new(big.Int).SetInt64(rpc.FinalizedBlockNumber.Int64())

// GethSettlement is the go-ethereum-backed implementation of
// SettlementLayer. It wraps go-ethereum's ethclient plus the abigen-
// generated StarknetFilterer to talk to the Starknet core L1 bridge.
//
// The same instance also satisfies rpccore.L1Client via
// TransactionReceipt, so node.go can construct one client and hand it
// to both the L1 sync loop and the RPC handlers.
//
// go-ethereum's rpc.Client transparently reconnects unary calls over
// the WS transport, and any active subscription propagates the drop
// via its Err() channel, which the upper-layer resubscribe loop in
// l1.Client.watchL1StateUpdates already handles.
type GethSettlement struct {
	contractAddress eth.Address
	url             string
	logger          log.StructuredLogger

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
	opts ...SettlementOption,
) (*GethSettlement, error) {
	o := settlementOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	logger := o.logger
	if logger == nil {
		logger = log.NewNopZapLogger()
	}

	rpcClient, err := rpc.DialContext(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("dial L1: %w", err)
	}
	ethClient := ethclient.NewClient(rpcClient)
	filterer, err := contract.NewStarknetFilterer(common.Address(contractAddress), ethClient)
	if err != nil {
		ethClient.Close()
		return nil, fmt.Errorf("bind Starknet filterer: %w", err)
	}

	return &GethSettlement{
		contractAddress: contractAddress,
		url:             rawURL,
		logger:          logger,
		rpcClient:       rpcClient,
		ethClient:       ethClient,
		filterer:        filterer,
		listener:        SelectiveListener{},
	}, nil
}

// SetListener swaps the event listener after construction. The metrics
// listener captures the settlement in a closure (so it can read gauges
// off it), which means the listener can only be built AFTER the
// settlement exists — hence a post-construction setter rather than a
// constructor option.
//
// Concurrency contract: SetListener MUST be called before the
// settlement is handed off to any goroutine (i.e. before l1.NewClient
// in node.go). The field is read unlocked from every RPC method; a
// concurrent SetListener would be a data race. The single-shot wiring
// in node.go satisfies this — don't introduce a second caller.
func (s *GethSettlement) SetListener(l EventListener) { s.listener = l }

// observe wraps an RPC call so OnL1Call fires on both success and
// failure paths — error rates and latency under failure are as
// interesting to monitor as success.
func (s *GethSettlement) observe(method string) func() {
	t := time.Now()
	return func() { s.listener.OnL1Call(method, time.Since(t)) }
}

// ChainID returns the Ethereum chain id (eth_chainId).
func (s *GethSettlement) ChainID(ctx context.Context) (*big.Int, error) {
	defer s.observe("eth_chainId")()
	id, err := s.ethClient.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get chain id: %w", err)
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
		return 0, fmt.Errorf("get finalised Ethereum block: %w", err)
	}
	return head.Number.Uint64(), nil
}

// LatestHeight returns the latest known L1 block number (eth_blockNumber).
func (s *GethSettlement) LatestHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_blockNumber")()
	n, err := s.ethClient.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("get latest Ethereum block number: %w", err)
	}
	return n, nil
}

// FilterStateUpdate decodes every LogStateUpdate in [from, to] into
// the chain-neutral StateUpdate shape.
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
		return nil, fmt.Errorf("filter LogStateUpdate [%d,%d]: %w", from, to, err)
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
) (eth.Subscription, error) {
	raw := make(chan *contract.StarknetLogStateUpdate, watchForwarderBuffer)
	inner, err := s.filterer.WatchLogStateUpdate(&bind.WatchOpts{Context: ctx}, raw)
	if err != nil {
		return nil, fmt.Errorf("subscribe LogStateUpdate: %w", err)
	}
	w := &gethStateUpdateForwarder{
		inner:  inner,
		sink:   sink,
		raw:    raw,
		errCh:  make(chan error, 1),
		closed: make(chan struct{}),
	}
	go w.run()
	return w, nil
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
			return nil, fmt.Errorf("get transaction receipt: %w", eth.ErrNotFound)
		}
		return nil, fmt.Errorf("get transaction receipt: %w", err)
	}
	return gethReceiptToEth(r), nil
}

// Close releases the underlying RPC client.
func (s *GethSettlement) Close() {
	s.ethClient.Close()
}

// stateUpdateFromGethContract translates the abigen-decoded event into
// the chain-neutral StateUpdate. felt conversion happens here so
// l1.Client never touches Ethereum-flavoured types.
func stateUpdateFromGethContract(ev *contract.StarknetLogStateUpdate) *StateUpdate {
	return &StateUpdate{
		L2BlockNumber: ev.BlockNumber.Uint64(),
		L2BlockHash:   new(felt.Felt).SetBigInt(ev.BlockHash),
		StateRoot:     new(felt.Felt).SetBigInt(ev.GlobalRoot),
		L1RefHeight:   ev.Raw.BlockNumber,
		Removed:       ev.Raw.Removed,
	}
}

// gethReceiptToEth shallow-copies the fields of a geth receipt that juno
// actually reads. Today only Logs is consumed, but every nested Log is
// converted so the shape matches the chain-neutral receipt type exactly.
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

// gethStateUpdateForwarder decodes contract.StarknetLogStateUpdate events
// into l1.StateUpdate as they arrive from the underlying log subscription.
type gethStateUpdateForwarder struct {
	inner     event.Subscription
	sink      chan<- *StateUpdate
	raw       chan *contract.StarknetLogStateUpdate
	errCh     chan error
	closed    chan struct{}
	closeOnce sync.Once
}

func (w *gethStateUpdateForwarder) Err() <-chan error { return w.errCh }

func (w *gethStateUpdateForwarder) Unsubscribe() {
	w.shutdown(nil)
	w.inner.Unsubscribe()
}

func (w *gethStateUpdateForwarder) shutdown(cause error) {
	w.closeOnce.Do(func() {
		close(w.closed)
		if cause != nil {
			select {
			case w.errCh <- cause:
			default:
			}
		}
		close(w.errCh)
	})
}

func (w *gethStateUpdateForwarder) run() {
	defer w.shutdown(nil)
	for {
		select {
		case <-w.closed:
			return
		case err, ok := <-w.inner.Err():
			if !ok {
				return
			}
			w.shutdown(err)
			return
		case ev := <-w.raw:
			su := stateUpdateFromGethContract(ev)
			select {
			case w.sink <- su:
			case <-w.closed:
				return
			}
		}
	}
}

// Compile-time assertions: GethSettlement satisfies both interfaces it
// is intended to serve.
var (
	_ SettlementLayer  = (*GethSettlement)(nil)
	_ rpccore.L1Client = (*GethSettlement)(nil)
)
