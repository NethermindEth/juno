package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// transport is the surface methods use to talk to the remote endpoint.
// Today the only implementation is wsTransport; the interface remains
// because it keeps WebSocket-specific state out of methods.go and makes
// the seam between the public Client and the wire layer testable.
type transport interface {
	call(ctx context.Context, method string, params ...any) (json.RawMessage, error)
	close()
}

// Client is the hand-rolled Ethereum execution-layer client juno uses to
// follow the L1 head and serve starknet_getMessageStatus. It speaks the
// minimum subset of the JSON-RPC surface juno needs and nothing more.
//
// Only WebSocket endpoints are supported: subscribe-based log delivery
// (eth_subscribe) requires a long-lived connection, and unary calls
// happily share that same connection.
type Client struct {
	tr transport
}

// New dials the endpoint at rawURL. The URL must use the ws:// or
// wss:// scheme.
func New(ctx context.Context, rawURL string) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("unsupported url scheme %q (need ws/wss)", u.Scheme)
	}
	ws, err := dialWS(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return &Client{tr: ws}, nil
}

// Close releases the underlying transport.
func (c *Client) Close() { c.tr.close() }
