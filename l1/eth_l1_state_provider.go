package l1

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/eth/contract"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils/log"
)

type ethL1StateProviderOptions struct {
	logger   log.StructuredLogger
	listener EventListener
}

type EthL1StateProviderOption func(*ethL1StateProviderOptions)

func WithEthL1StateProviderLogger(l log.StructuredLogger) EthL1StateProviderOption {
	return func(o *ethL1StateProviderOptions) { o.logger = l }
}

func WithEthL1StateProviderListener(l EventListener) EthL1StateProviderOption {
	return func(o *ethL1StateProviderOptions) { o.listener = l }
}

// EthL1StateProvider serves both the L1 sync loop (L1StateProvider) and the
// RPC handlers (rpccore.L1Client). Connection drops are recovered by the
// underlying client; a dropped live subscription surfaces on Err() for
// l1.Client to resubscribe.
type EthL1StateProvider struct {
	client          *client.Client
	contractAddress eth.Address
	listener        EventListener
}

func NewEthL1StateProvider(
	ctx context.Context,
	url string,
	contractAddress eth.Address,
	opts ...EthL1StateProviderOption,
) (*EthL1StateProvider, error) {
	o := ethL1StateProviderOptions{listener: SelectiveListener{}}
	for _, opt := range opts {
		opt(&o)
	}
	clientOpts := []client.Option{}
	if o.logger != nil {
		clientOpts = append(clientOpts, client.WithLogger(o.logger))
	}
	c, err := client.New(ctx, url, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("dialing L1: %w", err)
	}
	return &EthL1StateProvider{
		client:          c,
		contractAddress: contractAddress,
		listener:        o.listener,
	}, nil
}

func (s *EthL1StateProvider) observe(method string) func() {
	t := time.Now()
	return func() { s.listener.OnL1Call(method, time.Since(t)) }
}

func (s *EthL1StateProvider) ChainID(ctx context.Context) (*big.Int, error) {
	defer s.observe("eth_chainId")()
	return s.client.ChainID(ctx)
}

// FinalisedHeight returns eth.ErrNotFound when the node hasn't seen finality
// yet, distinguishing that from a transport failure.
func (s *EthL1StateProvider) FinalisedHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_getBlockByNumber")()
	h, err := s.client.HeaderByNumber(ctx, client.BlockFinalized)
	if err != nil {
		return 0, fmt.Errorf("getting finalised block: %w", err)
	}
	return uint64(h.Number), nil
}

func (s *EthL1StateProvider) LatestHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_blockNumber")()
	return s.client.BlockNumber(ctx)
}

func (s *EthL1StateProvider) FilterStateUpdate(
	ctx context.Context,
	from, to uint64,
) ([]*StateUpdate, error) {
	defer s.observe("eth_getLogs")()
	events, err := contract.FilterLogStateUpdate(ctx, s.client, s.contractAddress, from, to)
	if err != nil {
		return nil, fmt.Errorf("filtering LogStateUpdate [%d,%d]: %w", from, to, err)
	}
	out := make([]*StateUpdate, len(events))
	for i, ev := range events {
		su := stateUpdateFromContract(ev)
		out[i] = &su
	}
	return out, nil
}

// WatchStateUpdate fails the subscription with client.ErrSubscriptionQueueOverflow
// when sink is not drained promptly.
func (s *EthL1StateProvider) WatchStateUpdate(
	ctx context.Context,
	sink chan<- *StateUpdate,
) (Subscription, error) {
	raw := make(chan *eth.Log, watchForwarderBuffer)
	inner, err := s.client.SubscribeLogs(ctx, contract.LogStateUpdateFilter(s.contractAddress), raw)
	if err != nil {
		return nil, fmt.Errorf("subscribing to LogStateUpdate: %w", err)
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

func (s *EthL1StateProvider) TransactionReceipt(
	ctx context.Context,
	txHash eth.Hash,
) (eth.Receipt, error) {
	defer s.observe("eth_getTransactionReceipt")()
	return s.client.TransactionReceipt(ctx, txHash)
}

func (s *EthL1StateProvider) Close() {
	s.client.Close()
}

func stateUpdateFromContract(ev *contract.LogStateUpdate) StateUpdate {
	return StateUpdate{
		L2BlockNumber: ev.BlockNumber,
		L2BlockHash:   ev.BlockHash,
		StateRoot:     ev.GlobalRoot,
		L1RefHeight:   uint64(ev.Raw.BlockNumber),
		Removed:       ev.Raw.Removed,
	}
}

type stateUpdateForwarder struct {
	inner     Subscription
	sink      chan<- *StateUpdate
	raw       chan *eth.Log
	errCh     chan error
	closed    chan struct{}
	closeOnce sync.Once
}

func (w *stateUpdateForwarder) Err() <-chan error { return w.errCh }

func (w *stateUpdateForwarder) Unsubscribe() {
	w.shutdown(nil)
	w.inner.Unsubscribe()
}

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
		case rawLog := <-w.raw:
			ev, err := contract.Decode(rawLog)
			if err != nil {
				w.shutdown(fmt.Errorf("decoding LogStateUpdate: %w", err))
				w.inner.Unsubscribe()
				return
			}
			su := stateUpdateFromContract(ev)
			select {
			case w.sink <- &su:
			case <-w.closed:
				return
			}
		}
	}
}

var (
	_ L1StateProvider  = (*EthL1StateProvider)(nil)
	_ rpccore.L1Client = (*EthL1StateProvider)(nil)
)
