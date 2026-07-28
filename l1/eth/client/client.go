package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"sync"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// ErrClosed is terminal: the client was explicitly closed and will not redial.
var ErrClosed = errors.New("client closed")

type Client struct {
	url  string
	opts options

	mu     sync.Mutex
	tr     *wsTransport
	closed bool

	dials singleflight.Group
}

const BlockFinalized = "finalized"

const maxQuantityNibbles = 64

func New(ctx context.Context, rawURL string, opts ...Option) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing url: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("unsupported url scheme %q (need ws/wss)", u.Scheme)
	}
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	if o.logger == nil {
		o.logger = log.NewNopZapLogger()
	}
	ws, err := dialWS(ctx, rawURL, o)
	if err != nil {
		return nil, err
	}
	return &Client{
		url:  rawURL,
		opts: o,
		tr:   ws,
	}, nil
}

// Close is terminal: no further redials.
func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	tr := c.tr
	c.mu.Unlock()
	tr.close()
}

func (c *Client) transport() (*wsTransport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}
	return c.tr, nil
}

func (c *Client) redial(ctx context.Context, stale *wsTransport) (*wsTransport, error) {
	ch := c.dials.DoChan("dial", func() (any, error) {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return nil, ErrClosed
		}
		if c.tr != stale {
			// Someone already replaced it; their transport is current.
			tr := c.tr
			c.mu.Unlock()
			return tr, nil
		}
		c.mu.Unlock()

		// A redialed transport serves all future callers, so no caller's ctx
		// may own it; Close retires it. dialWS bounds the dial attempt itself.
		tr, err := dialWS(context.Background(), c.url, c.opts)
		if err != nil {
			c.opts.logger.Warn("redial failed", zap.Error(err))
			return nil, fmt.Errorf("redialing: %w", err)
		}
		c.mu.Lock()
		if c.closed {
			// Close raced the dial; don't leak the fresh transport.
			c.mu.Unlock()
			tr.close()
			return nil, ErrClosed
		}
		c.tr = tr
		c.mu.Unlock()
		return tr, nil
	})
	select {
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}
		return res.Val.(*wsTransport), nil
	case <-ctx.Done():
		// The in-flight dial continues for other waiters; only this caller bails.
		return nil, ctx.Err()
	}
}

func withRedial[T any](ctx context.Context, c *Client, fn func(*wsTransport) (T, error)) (T, error) {
	var zero T
	tr, err := c.transport()
	if err != nil {
		return zero, err
	}
	out, err := fn(tr)
	if !errors.Is(err, ErrTransportClosed) {
		return out, err
	}
	tr, err = c.redial(ctx, tr)
	if err != nil {
		return zero, err
	}
	return fn(tr)
}

func (c *Client) call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	return withRedial(ctx, c, func(tr *wsTransport) (json.RawMessage, error) {
		return tr.call(ctx, method, params...)
	})
}

// SubscribeLogs uses ctx only for the subscribe call itself — afterwards the
// sub lives until Unsubscribe or a transport failure (go-ethereum semantics).
func (c *Client) SubscribeLogs(
	ctx context.Context,
	q FilterQuery,
	sink chan<- *eth.Log,
) (Subscription, error) {
	return withRedial(ctx, c, func(tr *wsTransport) (Subscription, error) {
		return tr.subscribeLogs(ctx, q, sink)
	})
}

func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	raw, err := c.call(ctx, "eth_chainId")
	if err != nil {
		return nil, fmt.Errorf("getting chain ID: %w", err)
	}
	return decodeQuantityBig(raw)
}

func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	raw, err := c.call(ctx, "eth_blockNumber")
	if err != nil {
		return 0, fmt.Errorf("getting block number: %w", err)
	}
	return decodeQuantityUint64(raw)
}

// HeaderByNumber maps a null result — the server's signal for "the named
// block does not exist yet" — to eth.ErrNotFound.
func (c *Client) HeaderByNumber(ctx context.Context, tag string) (eth.Header, error) {
	raw, err := c.call(ctx, "eth_getBlockByNumber", tag, false /* hydrated txs */)
	if err != nil {
		return eth.Header{}, fmt.Errorf("getting block: %w", err)
	}
	if isJSONNull(raw) {
		return eth.Header{}, eth.ErrNotFound
	}
	var h eth.Header
	if err := json.Unmarshal(raw, &h); err != nil {
		return eth.Header{}, fmt.Errorf("decoding header: %w", err)
	}
	return h, nil
}

// TransactionReceipt maps a null result (receipt unknown to the node) to
// eth.ErrNotFound.
func (c *Client) TransactionReceipt(ctx context.Context, txHash eth.Hash) (eth.Receipt, error) {
	raw, err := c.call(ctx, "eth_getTransactionReceipt", txHash)
	if err != nil {
		return eth.Receipt{}, fmt.Errorf("getting receipt: %w", err)
	}
	if isJSONNull(raw) {
		return eth.Receipt{}, eth.ErrNotFound
	}
	var r eth.Receipt
	if err := json.Unmarshal(raw, &r); err != nil {
		return eth.Receipt{}, fmt.Errorf("decoding receipt: %w", err)
	}
	return r, nil
}

func (c *Client) FilterLogs(ctx context.Context, q FilterQuery) ([]eth.Log, error) {
	raw, err := c.call(ctx, "eth_getLogs", q)
	if err != nil {
		return nil, fmt.Errorf("filtering logs: %w", err)
	}
	if isJSONNull(raw) {
		return nil, nil
	}
	var logs []eth.Log
	if err := json.Unmarshal(raw, &logs); err != nil {
		return nil, fmt.Errorf("decoding logs: %w", err)
	}
	return logs, nil
}

func decodeQuantityUint64(raw json.RawMessage) (uint64, error) {
	var q eth.HexU64
	if err := json.Unmarshal(raw, &q); err != nil {
		return 0, fmt.Errorf("decoding quantity: %w", err)
	}
	return uint64(q), nil
}

func decodeQuantityBig(raw json.RawMessage) (*big.Int, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decoding quantity: %w", err)
	}
	if len(s) < 2 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return nil, fmt.Errorf("decoding quantity: missing 0x prefix in %q", s)
	}
	body := s[2:]
	if body == "" {
		return nil, fmt.Errorf("decoding quantity: no digits in %q", s)
	}
	// 64 nibbles (256 bits) is generous for any real quantity; without a cap a
	// hostile endpoint could make us allocate a big.Int up to the read limit.
	if len(body) > maxQuantityNibbles {
		return nil, fmt.Errorf("decoding quantity: too long (%d nibbles)", len(body))
	}
	if body[0] == '+' || body[0] == '-' {
		return nil, fmt.Errorf("decoding quantity: sign prefix in %q", s)
	}
	if len(body) > 1 && body[0] == '0' {
		return nil, fmt.Errorf("decoding quantity: leading zero in %q", s)
	}
	out, ok := new(big.Int).SetString(body, 16)
	if !ok {
		return nil, fmt.Errorf("decoding quantity: invalid hex %q", s)
	}
	return out, nil
}
