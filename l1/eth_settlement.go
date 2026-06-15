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
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
)

// ErrSettlementClosed is returned when an EthSettlement method is
// invoked after Close. Distinct from client.ErrTransportClosed: the
// latter is a transient state we recover from by redialing; this one
// is terminal.
var ErrSettlementClosed = errors.New("settlement closed")

// watchForwarderBuffer is the per-subscription buffer between the
// contract decoder and the l1.StateUpdate sink consumed by l1.Client.
const watchForwarderBuffer = 64

// EthSettlement is the Ethereum implementation of SettlementLayer. It
// wraps a hand-rolled JSON-RPC client (l1/eth/client) and the hand-
// written LogStateUpdate decoder (l1/eth/contract) — together they
// replace the go-ethereum ethclient + abigen pipeline.
//
// The same instance also satisfies rpccore.L1Client via TransactionReceipt,
// so node.go can construct one client and hand it to both the L1 sync
// loop and the RPC handlers.
//
// EthSettlement also keeps the connection details so a dropped WS conn
// can be transparently redialed. The hand-rolled client is one-shot:
// once its transport reports closed, every subsequent call returns
// client.ErrTransportClosed. We catch that, redial, and retry once —
// upper layers (l1.Client.subscribeToUpdates) just see their next call
// succeed without ever knowing the conn flapped. This matches what
// go-ethereum's rpc.Client does internally (it transparently
// reconnects; subscriptions still need re-issuing, same as here).
type EthSettlement struct {
	contractAddress eth.Address
	url             string
	clientOpts      []client.Option
	logger          log.StructuredLogger
	listener        EventListener

	mu     sync.Mutex // protects client and closed
	client *client.Client
	closed bool
}

// NewEthSettlement dials the Ethereum endpoint at url and returns a
// ready-to-use settlement-layer adapter bound to contractAddress
// (the Starknet core L1 bridge). The transport is selected by URL
// scheme; ws/wss is required if the caller intends to use
// WatchStateUpdate. The url and any client options are remembered so
// dropped connections can be redialed transparently.
func NewEthSettlement(
	ctx context.Context,
	url string,
	contractAddress eth.Address,
	opts ...EthSettlementOption,
) (*EthSettlement, error) {
	o := ethSettlementOptions{}
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
		return nil, fmt.Errorf("dial L1: %w", err)
	}
	s := &EthSettlement{
		client:          c,
		contractAddress: contractAddress,
		url:             url,
		clientOpts:      clientOpts,
		logger:          logger,
		listener:        SelectiveListener{},
	}
	return s, nil
}

// EthSettlementOption configures an EthSettlement at construction time.
type EthSettlementOption func(*ethSettlementOptions)

type ethSettlementOptions struct {
	logger log.StructuredLogger
}

// WithSettlementLogger attaches a logger forwarded to the underlying
// JSON-RPC client. Surfaces dropped frames and best-effort
// eth_unsubscribe failures at debug level.
func WithSettlementLogger(l log.StructuredLogger) EthSettlementOption {
	return func(o *ethSettlementOptions) { o.logger = l }
}

// SetListener swaps the event listener after construction. The metrics
// listener captures the settlement in a closure (so it can read gauges
// off it), which means the listener can only be built AFTER the
// settlement exists — hence a post-construction setter rather than a
// constructor option.
//
// Concurrency contract: SetListener MUST be called before the
// settlement is handed off to any goroutine (i.e. before NewClient(...)
// in node.go). The field is read unlocked from every RPC method; a
// concurrent SetListener would be a data race. The single-shot wiring
// in node.go satisfies this — don't introduce a second caller.
func (s *EthSettlement) SetListener(l EventListener) { s.listener = l }

// observe wraps an RPC call so OnL1Call fires on both success and
// failure paths — error rates and latency under failure are as
// interesting to monitor as success.
func (s *EthSettlement) observe(method string) func() {
	t := time.Now()
	return func() { s.listener.OnL1Call(method, time.Since(t)) }
}

// currentClient returns the active *client.Client. Callers must call
// redial(stale) if they observe client.ErrTransportClosed from any
// call made against the returned client.
func (s *EthSettlement) currentClient() (*client.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrSettlementClosed
	}
	return s.client, nil
}

// redial closes the stale client (idempotent) and dials a new one,
// unless another caller already redialed first. Returns the active
// client after the operation completes. Concurrent callers see one
// successful redial; the rest pick up the new client without dialing
// again.
func (s *EthSettlement) redial(ctx context.Context, stale *client.Client) (*client.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrSettlementClosed
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
		return nil, fmt.Errorf("redial L1: %w", err)
	}
	s.client = c
	return c, nil
}

// withRetryOnClosed calls fn against the current client; if fn returns
// client.ErrTransportClosed it redials once and retries. Other errors
// (including ctx cancellation) bubble up unchanged.
func withRetryOnClosed[T any](
	ctx context.Context,
	s *EthSettlement,
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

// ChainID returns the Ethereum chain id (eth_chainId).
func (s *EthSettlement) ChainID(ctx context.Context) (*big.Int, error) {
	defer s.observe("eth_chainId")()
	id, err := withRetryOnClosed(ctx, s, func(c *client.Client) (*big.Int, error) {
		return c.ChainID(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("get chain id: %w", err)
	}
	return id, nil
}

// FinalisedHeight returns the latest finalised L1 block number. A
// missing finalised header is reported as eth.ErrNotFound so callers
// can distinguish "node hasn't seen finality yet" from a transport
// failure.
func (s *EthSettlement) FinalisedHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_getBlockByNumber")()
	h, err := withRetryOnClosed(ctx, s, func(c *client.Client) (*eth.Header, error) {
		return c.HeaderByNumber(ctx, client.BlockFinalized)
	})
	if err != nil {
		if errors.Is(err, eth.ErrNotFound) {
			return 0, fmt.Errorf("finalised block not found: %w", eth.ErrNotFound)
		}
		return 0, fmt.Errorf("get finalised Ethereum block: %w", err)
	}
	return uint64(h.Number), nil
}

// LatestHeight returns the latest known L1 block number (eth_blockNumber).
func (s *EthSettlement) LatestHeight(ctx context.Context) (uint64, error) {
	defer s.observe("eth_blockNumber")()
	n, err := withRetryOnClosed(ctx, s, func(c *client.Client) (uint64, error) {
		return c.BlockNumber(ctx)
	})
	if err != nil {
		return 0, fmt.Errorf("get latest Ethereum block number: %w", err)
	}
	return n, nil
}

// FilterStateUpdate decodes every LogStateUpdate in [from, to] into
// the chain-neutral StateUpdate shape.
func (s *EthSettlement) FilterStateUpdate(
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
		return nil, fmt.Errorf("filter LogStateUpdate [%d,%d]: %w", from, to, err)
	}
	out := make([]*StateUpdate, len(events))
	for i, ev := range events {
		out[i] = stateUpdateFromContract(ev)
	}
	return out, nil
}

// WatchStateUpdate subscribes to live LogStateUpdate events and
// forwards each one (decoded into StateUpdate, with felt conversion
// already applied) on sink. Requires a ws/wss endpoint. Redials the
// underlying transport if it has been closed.
func (s *EthSettlement) WatchStateUpdate(
	ctx context.Context,
	sink chan<- *StateUpdate,
) (eth.Subscription, error) {
	raw := make(chan *contract.LogStateUpdate, watchForwarderBuffer)
	inner, err := withRetryOnClosed(ctx, s, func(c *client.Client) (eth.Subscription, error) {
		return contract.WatchLogStateUpdate(ctx, c, s.contractAddress, raw)
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

// TransactionReceipt fetches an L1 transaction receipt by hash. Used by
// the RPC handlers for starknet_getMessageStatus.
func (s *EthSettlement) TransactionReceipt(
	ctx context.Context,
	txHash eth.Hash,
) (*eth.Receipt, error) {
	defer s.observe("eth_getTransactionReceipt")()
	r, err := withRetryOnClosed(ctx, s, func(c *client.Client) (*eth.Receipt, error) {
		return c.TransactionReceipt(ctx, txHash)
	})
	if err != nil {
		return nil, fmt.Errorf("get transaction receipt: %w", err)
	}
	return r, nil
}

// Close marks the settlement as terminally closed (no further
// redials) and releases the active transport.
func (s *EthSettlement) Close() {
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

// stateUpdateFromContract translates the on-chain event shape into the
// chain-neutral StateUpdate. The contract decoder already lands felts
// and uint64s in their target types, so this is just a field rename.
func stateUpdateFromContract(ev *contract.LogStateUpdate) *StateUpdate {
	return &StateUpdate{
		L2BlockNumber: ev.BlockNumber,
		L2BlockHash:   &ev.BlockHash,
		StateRoot:     &ev.GlobalRoot,
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

// shutdown is the single termination path for this forwarder. cause
// is nil for a clean teardown (Unsubscribe or normal run() exit) and
// non-nil when the inner subscription emitted an error — in that case
// it is delivered on Err() before the channel is closed. sync.Once
// makes concurrent calls (Unsubscribe + run's deferred close) safe.
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
