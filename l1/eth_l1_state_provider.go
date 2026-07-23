package l1

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
	"github.com/NethermindEth/juno/l1/eth/contract"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// ErrClosed is terminal, unlike client.ErrTransportClosed which is recovered
// by a redial.
var ErrClosed = errors.New("L1 state provider closed")

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
// RPC handlers (rpccore.L1Client), redialing dropped connections.
type EthL1StateProvider struct {
	contractAddress eth.Address
	url             string
	clientOpts      []client.Option
	logger          log.StructuredLogger
	listener        EventListener

	mu     sync.Mutex // protects client and closed
	client *client.Client
	closed bool
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
	logger := o.logger
	if logger == nil {
		logger = log.NewNopZapLogger()
	}
	clientOpts := []client.Option{client.WithLogger(logger)}
	c, err := client.New(ctx, url, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("dialing L1: %w", err)
	}
	s := &EthL1StateProvider{
		client:          c,
		contractAddress: contractAddress,
		url:             url,
		clientOpts:      clientOpts,
		logger:          logger,
		listener:        o.listener,
	}
	return s, nil
}

func (s *EthL1StateProvider) observe(method string) func() {
	t := time.Now()
	return func() { s.listener.OnL1Call(method, time.Since(t)) }
}

func (s *EthL1StateProvider) currentClient() (*client.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	return s.client, nil
}

// redial coalesces concurrent callers: losers pick up the winner's client
// without dialing again.
func (s *EthL1StateProvider) redial(
	ctx context.Context, stale *client.Client,
) (*client.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if s.client != stale {
		// Someone else won the race; their fresh client is current.
		return s.client, nil
	}
	stale.Close()
	s.logger.Info("L1 transport closed; redialing")
	c, err := client.New(ctx, s.url, s.clientOpts...)
	if err != nil {
		s.logger.Trace("L1 redial failed", zap.Error(err))
		return nil, fmt.Errorf("redialing L1: %w", err)
	}
	s.client = c
	return c, nil
}

// withRetryOnClosed redials once and retries fn on client.ErrTransportClosed —
// the underlying client is one-shot. Other errors bubble up unchanged.
func withRetryOnClosed[T any](
	ctx context.Context,
	s *EthL1StateProvider,
	fn func(*client.Client) (T, error),
) (T, error) {
	var zero T
	c, err := s.currentClient()
	if err != nil {
		return zero, err
	}
	out, err := fn(c)
	if !errors.Is(err, client.ErrTransportClosed) {
		return out, err
	}
	c2, rdErr := s.redial(ctx, c)
	if rdErr != nil {
		return zero, rdErr
	}
	return fn(c2)
}

func (s *EthL1StateProvider) ChainID(ctx context.Context) (*big.Int, error) {
	defer s.observe("eth_chainId")()
	return withRetryOnClosed(ctx, s, func(c *client.Client) (*big.Int, error) {
		return c.ChainID(ctx)
	})
}

// FinalisedHeight returns eth.ErrNotFound when the node hasn't seen finality
// yet, distinguishing that from a transport failure.
func (s *EthL1StateProvider) FinalisedHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_getBlockByNumber")()
	h, err := withRetryOnClosed(ctx, s, func(c *client.Client) (*eth.Header, error) {
		return c.HeaderByNumber(ctx, client.BlockFinalized)
	})
	if err != nil {
		return 0, err
	}
	return uint64(h.Number), nil
}

func (s *EthL1StateProvider) LatestHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_blockNumber")()
	return withRetryOnClosed(ctx, s, func(c *client.Client) (uint64, error) {
		return c.BlockNumber(ctx)
	})
}

func (s *EthL1StateProvider) FilterStateUpdate(
	ctx context.Context,
	from, to uint64,
) ([]*StateUpdate, error) {
	defer s.observe("eth_getLogs")()
	events, err := withRetryOnClosed(
		ctx,
		s,
		func(c *client.Client) ([]*contract.LogStateUpdate, error) {
			return contract.FilterLogStateUpdate(ctx, c, s.contractAddress, from, to)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("filtering LogStateUpdate [%d,%d]: %w", from, to, err)
	}
	out := make([]*StateUpdate, len(events))
	for i, ev := range events {
		out[i] = stateUpdateFromContract(ev)
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
	inner, err := withRetryOnClosed(ctx, s, func(c *client.Client) (Subscription, error) {
		return c.SubscribeLogs(ctx, contract.LogStateUpdateFilter(s.contractAddress), raw)
	})
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

func (s *EthL1StateProvider) TransactionReceipt(
	ctx context.Context,
	txHash eth.Hash,
) (eth.Receipt, error) {
	defer s.observe("eth_getTransactionReceipt")()
	r, err := withRetryOnClosed(ctx, s, func(c *client.Client) (*eth.Receipt, error) {
		return c.TransactionReceipt(ctx, txHash)
	})
	if err != nil {
		return eth.Receipt{}, err
	}
	return *r, nil
}

// Close is terminal: no further redials.
func (s *EthL1StateProvider) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.client != nil {
		s.client.Close()
	}
}

func stateUpdateFromContract(ev *contract.LogStateUpdate) *StateUpdate {
	return &StateUpdate{
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
				return
			}
			su := stateUpdateFromContract(ev)
			select {
			case w.sink <- su:
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
