package starknet_client

import (
	"context"
	"github.com/ethereum/go-ethereum/rpc"
)

// Client defines typed wrappers for the StarkNet RPC API.
type Client struct {
	c *rpc.Client
}

// Dial connects a client to the given URL.
func Dial(rawUrl string) (*Client, error) {
	return DialContext(context.Background(), rawUrl)
}

func DialContext(ctx context.Context, rawUrl string) (*Client, error) {
	c, err := rpc.DialContext(ctx, rawUrl)
	if err != nil {
		return nil, err
	}
	return NewClient(c), nil
}

// NewClient creates a client that uses the given RPC client.
func NewClient(c *rpc.Client) *Client {
	return &Client{c}
}

func (ec *Client) Close() {
	ec.c.Close()
}

// StarkNet Access

// GetBlockByHash Get block information given the block id
func (ec *Client) GetBlockByHash(ctx context.Context, blockHash BlockHash, requestedScope RequestedScope) (*BlockResponse, error) {
	var result BlockResponse
	err := ec.c.CallContext(ctx, &result, "starknet_getBlockByHash", blockHash, requestedScope)
	if err != nil {
		return nil, err
	}
	return &result, err
}

// GetBlockByNumber Get block information given the block number
func (ec *Client) GetBlockByNumber(ctx context.Context, blockHash BlockHash, requestedScope RequestedScope) (*BlockResponse, error) {
	var result BlockResponse
	err := ec.c.CallContext(ctx, &result, "starknet_getBlockByNumber", blockHash, requestedScope)
	if err != nil {
		return nil, err
	}
	return &result, err
}
