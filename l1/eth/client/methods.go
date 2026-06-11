package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/l1/eth"
)

// Common block tags accepted by HeaderByNumber. Mirrors the post-merge
// JSON-RPC vocabulary; juno only uses Finalized today.
const (
	BlockFinalized = "finalized"
	BlockLatest    = "latest"
	BlockSafe      = "safe"
	BlockEarliest  = "earliest"
	BlockPending   = "pending"
)

// jsonNull is the literal payload returned by an RPC server for a
// missing resource (block, header, receipt). The methods layer maps
// this to eth.ErrNotFound on a per-method basis.
var jsonNull = []byte("null")

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), jsonNull)
}

// ChainID returns the chain identifier reported by eth_chainId.
func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	raw, err := c.tr.call(ctx, "eth_chainId")
	if err != nil {
		return nil, fmt.Errorf("eth_chainId: %w", err)
	}
	return decodeQuantityBig(raw)
}

// BlockNumber returns the latest known block number (eth_blockNumber).
func (c *Client) BlockNumber(ctx context.Context) (uint64, error) {
	raw, err := c.tr.call(ctx, "eth_blockNumber")
	if err != nil {
		return 0, fmt.Errorf("eth_blockNumber: %w", err)
	}
	return decodeQuantityUint64(raw)
}

// HeaderByNumber retrieves a block header by tag (one of BlockFinalized,
// BlockLatest, etc.). Block-number-specific lookups can be added when a
// caller needs them; juno only fetches the finalised head today.
//
// Returns eth.ErrNotFound if the remote replies with a null result (which
// is geth's signal for "the named block does not exist yet").
func (c *Client) HeaderByNumber(ctx context.Context, tag string) (*eth.Header, error) {
	raw, err := c.tr.call(ctx, "eth_getBlockByNumber", tag, false /* hydrated txs */)
	if err != nil {
		return nil, fmt.Errorf("eth_getBlockByNumber: %w", err)
	}
	if isJSONNull(raw) {
		return nil, eth.ErrNotFound
	}
	var h eth.Header
	if err := json.Unmarshal(raw, &h); err != nil {
		return nil, fmt.Errorf("eth_getBlockByNumber: decode header: %w", err)
	}
	return &h, nil
}

// TransactionReceipt fetches a transaction receipt by hash. Returns
// eth.ErrNotFound if the remote does not have the receipt.
func (c *Client) TransactionReceipt(ctx context.Context, txHash eth.Hash) (*eth.Receipt, error) {
	raw, err := c.tr.call(ctx, "eth_getTransactionReceipt", txHash)
	if err != nil {
		return nil, fmt.Errorf("eth_getTransactionReceipt: %w", err)
	}
	if isJSONNull(raw) {
		return nil, eth.ErrNotFound
	}
	var r eth.Receipt
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("eth_getTransactionReceipt: decode receipt: %w", err)
	}
	return &r, nil
}

// FilterLogs runs eth_getLogs with q. Empty result is not an error;
// returns an empty slice.
func (c *Client) FilterLogs(ctx context.Context, q FilterQuery) ([]eth.Log, error) {
	raw, err := c.tr.call(ctx, "eth_getLogs", q)
	if err != nil {
		return nil, fmt.Errorf("eth_getLogs: %w", err)
	}
	if isJSONNull(raw) {
		return nil, nil
	}
	var logs []eth.Log
	if err := json.Unmarshal(raw, &logs); err != nil {
		return nil, fmt.Errorf("eth_getLogs: decode logs: %w", err)
	}
	return logs, nil
}

// decodeQuantityUint64 parses a JSON-RPC "quantity" (0x-prefixed
// minimal hex string) into uint64.
func decodeQuantityUint64(raw json.RawMessage) (uint64, error) {
	var q eth.HexU64
	if err := json.Unmarshal(raw, &q); err != nil {
		return 0, fmt.Errorf("decode quantity: %w", err)
	}
	return uint64(q), nil
}

// decodeQuantityBig parses a JSON-RPC "quantity" into *big.Int. Chain
// IDs (and other uint256-shaped quantities) can exceed 64 bits, so we
// don't reuse the uint64 decoder.
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
