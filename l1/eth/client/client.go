package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"time"

	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/log"
)

// Only WebSocket endpoints are supported: subscribe-based log delivery
// (eth_subscribe) requires a long-lived connection, and unary calls
// happily share that same connection.
type Client struct {
	tr *wsTransport
}

const BlockFinalized = "finalized"

// The methods layer maps this to eth.ErrNotFound on a per-method basis.
var jsonNull = []byte("null")

type Option func(*options)

type options struct {
	logger       log.StructuredLogger
	pingInterval time.Duration
	pingTimeout  time.Duration
}

// Surfaces dropped frames, id mismatches, and best-effort eth_unsubscribe
// failures — events that would otherwise vanish into call timeouts.
func WithLogger(l log.StructuredLogger) Option {
	return func(o *options) { o.logger = l }
}

// WithPingConfig overrides the websocket keep-alive ping interval and the
// per-ping write/read timeout. The defaults (30s interval, 10s timeout)
// suit every production RPC endpoint; overriding is mainly useful in tests
// that need to observe ping behaviour without waiting out the real cadence.
// Non-positive values fall back to the defaults.
func WithPingConfig(interval, timeout time.Duration) Option {
	return func(o *options) {
		o.pingInterval = interval
		o.pingTimeout = timeout
	}
}

// URL must use the ws:// or wss:// scheme.
func New(ctx context.Context, rawURL string, opts ...Option) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("unsupported url scheme %q (need ws/wss)", u.Scheme)
	}
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	ws, err := dialWS(ctx, rawURL, o)
	if err != nil {
		return nil, err
	}
	return &Client{tr: ws}, nil
}

func (c *Client) Close() { c.tr.close() }

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), jsonNull)
}

func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	raw, err := c.tr.call(ctx, "eth_chainId")
	if err != nil {
		return nil, fmt.Errorf("getting chain ID: %w", err)
	}
	return decodeQuantityBig(raw)
}

func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	raw, err := c.tr.call(ctx, "eth_blockNumber")
	if err != nil {
		return 0, fmt.Errorf("getting block number: %w", err)
	}
	return decodeQuantityUint64(raw)
}

// Returns eth.ErrNotFound if the remote replies with a null result (geth's
// signal for "the named block does not exist yet").
func (c *Client) HeaderByNumber(ctx context.Context, tag string) (*eth.Header, error) {
	raw, err := c.tr.call(ctx, "eth_getBlockByNumber", tag, false /* hydrated txs */)
	if err != nil {
		return nil, fmt.Errorf("getting block: %w", err)
	}
	if isJSONNull(raw) {
		return nil, eth.ErrNotFound
	}
	var h eth.Header
	if err := json.Unmarshal(raw, &h); err != nil {
		return nil, fmt.Errorf("decoding header: %w", err)
	}
	return &h, nil
}

// Returns eth.ErrNotFound if the remote does not have the receipt.
func (c *Client) TransactionReceipt(ctx context.Context, txHash eth.Hash) (*eth.Receipt, error) {
	raw, err := c.tr.call(ctx, "eth_getTransactionReceipt", txHash)
	if err != nil {
		return nil, fmt.Errorf("getting receipt: %w", err)
	}
	if isJSONNull(raw) {
		return nil, eth.ErrNotFound
	}
	var r eth.Receipt
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("decoding receipt: %w", err)
	}
	return &r, nil
}

// Empty result is not an error; returns nil.
func (c *Client) FilterLogs(ctx context.Context, q FilterQuery) ([]eth.Log, error) {
	raw, err := c.tr.call(ctx, "eth_getLogs", q)
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
		return 0, fmt.Errorf("decode quantity: %w", err)
	}
	return uint64(q), nil
}

// Chain IDs can exceed 64 bits, so we can't reuse the uint64 decoder.
func decodeQuantityBig(raw json.RawMessage) (*big.Int, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decode quantity: %w", err)
	}
	if len(s) < 2 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return nil, fmt.Errorf("decode quantity: missing 0x prefix in %q", s)
	}
	body := s[2:]
	if body == "" {
		return nil, fmt.Errorf("decode quantity: no digits in %q", s)
	}
	if len(body) > 1 && body[0] == '0' {
		return nil, fmt.Errorf("decode quantity: leading zero in %q", s)
	}
	out, ok := new(big.Int).SetString(body, 16)
	if !ok {
		return nil, fmt.Errorf("decode quantity: invalid hex %q", s)
	}
	return out, nil
}
