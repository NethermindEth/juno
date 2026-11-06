package client

import (
	"context"
	"encoding/json"
)

type Client struct {
	tr *wsTransport
}

func New(ctx context.Context, rawURL string, opts ...Option) (*Client, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	tr, err := dialWS(ctx, rawURL, o)
	if err != nil {
		return nil, err
	}
	return &Client{tr: tr}, nil
}

func (c *Client) Close() { c.tr.close() }

// Call returns the raw JSON result of a JSON-RPC call.
func (c *Client) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	return c.tr.call(ctx, method, params...)
}
