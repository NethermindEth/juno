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

// ErrClosed is returned by methods invoked after Close. Unlike client.ErrTransportClosed
// (transient, recovered by redial), this is terminal.
var ErrClosed = errors.New("L1 state provider closed")

type EthL1StateProviderOption func(*ethL1StateProviderOptions)

type ethL1StateProviderOptions struct {
	logger   log.StructuredLogger
	listener EventListener
}

// Surfaces transport-level warnings at debug level.
func WithEthL1StateProviderLogger(l log.StructuredLogger) EthL1StateProviderOption {
	return func(o *ethL1StateProviderOptions) { o.logger = l }
}

// Must be called before the provider is handed off to any goroutine.
func WithEthL1StateProviderListener(l EventListener) EthL1StateProviderOption {
	return func(o *ethL1StateProviderOptions) { o.listener = l }
}

// Also satisfies rpccore.L1Client (via TransactionReceipt), so one instance serves both the sync
// loop and the RPC handlers. Keeps the dial details to transparently redial a dropped conn: the
// underlying client is one-shot, so on client.ErrTransportClosed we redial and retry once.
// Subscriptions still need re-issuing by the caller, same as go-ethereum's rpc.Client.
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

// ws/wss is required if the caller intends to use WatchStateUpdate. The url and
// any client options are remembered so dropped connections can be redialed transparently.
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

// observe fires OnL1Call on both success and failure — failure latency/rates matter too.
func (s *EthL1StateProvider) observe(method string) func() {
	t := time.Now()
	return func() { s.listener.OnL1Call(method, time.Since(t)) }
}

// Callers must call redial(stale) if they observe client.ErrTransportClosed.
func (s *EthL1StateProvider) currentClient() (*client.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	return s.client, nil
}

// Returns the active client after the operation. Concurrent callers see one
// successful redial; the rest pick up the new client without dialing again.
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

// withRetryOnClosed calls fn against the current client; if fn returns
// client.ErrTransportClosed it redials once and retries. Other errors
// (including ctx cancellation) bubble up unchanged.
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
	// The client layer already annotates the error ("getting chain ID: …").
	return withRetryOnClosed(ctx, s, func(c *client.Client) (*big.Int, error) {
		return c.ChainID(ctx)
	})
}

// A missing finalised header is reported as eth.ErrNotFound so callers
// can distinguish "node hasn't seen finality yet" from a transport failure.
func (s *EthL1StateProvider) FinalisedHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_getBlockByNumber")()
	h, err := withRetryOnClosed(ctx, s, func(c *client.Client) (*eth.Header, error) {
		return c.HeaderByNumber(ctx, client.BlockFinalized)
	})
	if err != nil {
		// eth.ErrNotFound passes through unwrapped; the client annotates the rest.
		return 0, err
	}
	return uint64(h.Number), nil
}

func (s *EthL1StateProvider) LatestHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_blockNumber")()
	// The client layer already annotates the error ("getting block number: …").
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

// Caller contract: sink MUST be drained promptly. A stalled sink back-pressures the
// transport's readLoop and stalls every unary RPC sharing the conn.
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
		// eth.ErrNotFound passes through unwrapped; the client annotates the rest.
		return eth.Receipt{}, err
	}
	return *r, nil
}

// Close is terminal — no further redials. Releases the active transport.
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

// The contract decoder already lands felts and uint64s in target types; no conversion needed.
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

// shutdown is the forwarder's single termination path. A non-nil cause is delivered on Err()
// before close; sync.Once makes concurrent calls (Unsubscribe + run's deferred close) safe.
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

// Compile-time interface assertions.
var (
	_ L1StateProvider  = (*EthL1StateProvider)(nil)
	_ rpccore.L1Client = (*EthL1StateProvider)(nil)
)
