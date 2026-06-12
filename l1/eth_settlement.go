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
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/eth/contract"
)

// watchForwarderBuffer is the per-subscription buffer between the
// contract decoder and the l1.StateUpdate sink consumed by l1.Client.
const watchForwarderBuffer = 64

// EthSettlement is the Ethereum implementation of SettlementLayer. It
// wraps a hand-rolled JSON-RPC client (l1/eth/client) and the hand-
// written LogStateUpdate decoder (l1/eth/contract) — together they
// replace the go-ethereum ethclient + abigen pipeline.
//
// The same instance also satisfies rpccore.L1Client /
// rpccore.EthMessagingClient via TransactionReceipt, so node.go can
// construct one client and hand it to both the L1 sync loop and the
// RPC handlers.
type EthSettlement struct {
	client          *client.Client
	contractAddress eth.Address
	listener        EventListener
}

// NewEthSettlement dials the Ethereum endpoint at url and returns a
// ready-to-use settlement-layer adapter bound to contractAddress
// (the Starknet core L1 bridge). The transport is selected by URL
// scheme; ws/wss is required if the caller intends to use
// WatchStateUpdate.
func NewEthSettlement(
	ctx context.Context, url string, contractAddress eth.Address, opts ...EthSettlementOption,
) (*EthSettlement, error) {
	c, err := client.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("dial L1: %w", err)
	}
	s := &EthSettlement{
		client:          c,
		contractAddress: contractAddress,
		listener:        SelectiveListener{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// EthSettlementOption configures an EthSettlement at construction time.
type EthSettlementOption func(*EthSettlement)

// WithSettlementListener attaches an EventListener used to emit the
// OnL1Call(method, duration) metric. Equivalent to SetListener.
func WithSettlementListener(l EventListener) EthSettlementOption {
	return func(s *EthSettlement) { s.listener = l }
}

// SetListener swaps the event listener after construction. Needed
// because the metrics listener closes over the settlement instance,
// so we have to build settlement first, then the listener, then wire
// them together.
func (s *EthSettlement) SetListener(l EventListener) { s.listener = l }

// ChainID returns the Ethereum chain id (eth_chainId).
func (s *EthSettlement) ChainID(ctx context.Context) (*big.Int, error) {
	t := time.Now()
	id, err := s.client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("get chain ID: %w", err)
	}
	s.listener.OnL1Call("eth_chainId", time.Since(t))
	return id, nil
}

// FinalisedHeight returns the latest finalised L1 block number. Maps
// eth.ErrNotFound from the underlying client to a descriptive error
// matching the legacy behaviour.
func (s *EthSettlement) FinalisedHeight(ctx context.Context) (uint64, error) {
	t := time.Now()
	h, err := s.client.HeaderByNumber(ctx, client.BlockFinalized)
	if err != nil {
		if errors.Is(err, eth.ErrNotFound) {
			s.listener.OnL1Call("eth_getBlockByNumber", time.Since(t))
			return 0, errors.New("finalised block not found")
		}
		return 0, fmt.Errorf("get finalised Ethereum block: %w", err)
	}
	s.listener.OnL1Call("eth_getBlockByNumber", time.Since(t))
	return uint64(h.Number), nil
}

// LatestHeight returns the latest known L1 block number (eth_blockNumber).
func (s *EthSettlement) LatestHeight(ctx context.Context) (uint64, error) {
	t := time.Now()
	n, err := s.client.BlockNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("get latest Ethereum block number: %w", err)
	}
	s.listener.OnL1Call("eth_blockNumber", time.Since(t))
	return n, nil
}

// FilterStateUpdate decodes every LogStateUpdate in [from, to] into
// the chain-neutral StateUpdate shape.
func (s *EthSettlement) FilterStateUpdate(
	ctx context.Context,
	from,
	to uint64,
) ([]*StateUpdate, error) {
	t := time.Now()
	events, err := contract.FilterLogStateUpdate(ctx, s.client, s.contractAddress, from, to)
	if err != nil {
		return nil, fmt.Errorf("filter LogStateUpdate [%d,%d]: %w", from, to, err)
	}
	s.listener.OnL1Call("eth_getLogs", time.Since(t))
	out := make([]*StateUpdate, len(events))
	for i, ev := range events {
		out[i] = stateUpdateFromContract(ev)
	}
	return out, nil
}

// WatchStateUpdate subscribes to live LogStateUpdate events and
// forwards each one (decoded into StateUpdate, with felt conversion
// already applied) on sink. Requires a ws/wss endpoint.
func (s *EthSettlement) WatchStateUpdate(
	ctx context.Context, sink chan<- *StateUpdate,
) (eth.Subscription, error) {
	raw := make(chan *contract.LogStateUpdate, watchForwarderBuffer)
	inner, err := contract.WatchLogStateUpdate(ctx, s.client, s.contractAddress, raw)
	if err != nil {
		return nil, err
	}
	w := &stateUpdateForwarder{
		inner:  inner,
		sink:   sink,
		raw:    raw,
		errCh:  make(chan error, 1),
		closed: make(chan struct{}),
	}
	go w.run()
	return w, nil
}

// TransactionReceipt fetches an L1 transaction receipt by hash. Used by
// the RPC handlers for starknet_getMessageStatus.
func (s *EthSettlement) TransactionReceipt(
	ctx context.Context,
	txHash eth.Hash,
) (*eth.Receipt, error) {
	t := time.Now()
	r, err := s.client.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("get eth transaction receipt: %w", err)
	}
	s.listener.OnL1Call("eth_getTransactionReceipt", time.Since(t))
	return r, nil
}

// Close releases the underlying transport.
func (s *EthSettlement) Close() { s.client.Close() }

// stateUpdateFromContract translates the on-chain event shape into the
// chain-neutral StateUpdate. felt conversion happens here so consumers
// (l1.Client) never see *big.Int.
func stateUpdateFromContract(ev *contract.LogStateUpdate) *StateUpdate {
	return &StateUpdate{
		L2BlockNumber: ev.BlockNumber.Uint64(),
		L2BlockHash:   new(felt.Felt).SetBigInt(ev.BlockHash),
		StateRoot:     new(felt.Felt).SetBigInt(ev.GlobalRoot),
		L1RefHeight:   uint64(ev.Raw.BlockNumber),
		Removed:       ev.Raw.Removed,
	}
}

// stateUpdateForwarder decodes contract.LogStateUpdate events into
// l1.StateUpdate as they arrive from the underlying log subscription.
type stateUpdateForwarder struct {
	inner     eth.Subscription
	sink      chan<- *StateUpdate
	raw       chan *contract.LogStateUpdate
	errCh     chan error
	closed    chan struct{}
	closeOnce sync.Once
}

func (w *stateUpdateForwarder) Err() <-chan error { return w.errCh }

func (w *stateUpdateForwarder) Unsubscribe() {
	w.shutdown(nil)
	w.inner.Unsubscribe()
}

// shutdown is the single termination path for this forwarder. cause is
// non-nil only when the inner subscription emitted an error; it gets
// delivered on Err() before close. sync.Once makes concurrent calls
// (Unsubscribe + run's deferred close) safe.
func (w *stateUpdateForwarder) shutdown(cause error) {
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

func (w *stateUpdateForwarder) run() {
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
			su := stateUpdateFromContract(ev)
			select {
			case w.sink <- su:
			case <-w.closed:
				return
			}
		}
	}
}

// Compile-time assertions: EthSettlement satisfies both interfaces it
// is intended to serve, and won't silently lose a method as the
// surface evolves.
var (
	_ SettlementLayer = (*EthSettlement)(nil)
)
