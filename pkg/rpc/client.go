package rpc

import (
	"context"

	grpc "github.com/ethereum/go-ethereum/rpc"
)

// Client is a wrapper around Go Ethereum's rpc.Client struct.
type Client struct {
	ptr *grpc.Client
}

// Close closes the client RPC connection.
func (c *Client) Close() {
	c.ptr.Close()
}

// GetBlockByHash gets block information by hash.
func (c *Client) GetBlockByHash(
	ctx context.Context, blockHash BlockHash, requestedScope RequestedScope,
) (*BlockResponse, error) {
	// var res BlockResponse
	// err := c.ptr.CallContext(
	// 	ctx, &res, "starknet_getBlockByHash", blockHash, requestedScope)
	// if err != nil {
	// 	return nil, err
	// }
	// return &res, err
	return &BlockResponse{}, nil
}

// GetBlockByNumber gets block information by block number.
func (ec *Client) GetBlockByNumber(
	ctx context.Context, blockHash BlockHash, requestedScope RequestedScope,
) (*BlockResponse, error) {
	// var res BlockResponse
	// err := ec.ptr.CallContext(
	// 	ctx, &res, "starknet_getBlockByNumber", blockHash, requestedScope)
	// if err != nil {
	// 	return nil, err
	// }
	// return &res, err
	return &BlockResponse{}, nil
}

// NewClient creates a new rpc.Client from a Go Ethereum's rpc.Client.
func NewClient(ptr *grpc.Client) *Client {
	return &Client{ptr}
}

// dialContext returns a new rpc.Client or returns an error otherwise.
func dialContext(ctx context.Context, rawUrl string) (*Client, error) {
	c, err := grpc.DialContext(ctx, rawUrl)
	if err != nil {
		return nil, err
	}
	return NewClient(c), nil
}

// Dial connects a client to the given URL.
func Dial(rawUrl string) (*Client, error) {
	return dialContext(context.Background(), rawUrl)
}
