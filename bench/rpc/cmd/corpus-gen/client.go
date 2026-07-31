package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type rpcClient struct {
	url    string
	client *http.Client
}

func newRPCClient(url string) *rpcClient {
	return &rpcClient{
		url:    url,
		client: &http.Client{Timeout: time.Minute},
	}
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("json-rpc error %d: %s", e.Code, e.Message)
}

type rpcEnvelope[T any] struct {
	Result T         `json:"result"`
	Error  *rpcError `json:"error"`
}

func rpcCall[T any](ctx context.Context, c *rpcClient, method string, params any) (T, error) {
	var zero T
	reqBody, err := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return zero, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(reqBody))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return zero, fmt.Errorf("%s: unexpected status %s: %s",
			method, resp.Status, bytes.TrimSpace(body))
	}

	var env rpcEnvelope[T]
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return zero, fmt.Errorf("%s: decode response: %w", method, err)
	}
	if env.Error != nil {
		return zero, fmt.Errorf("%s: %w", method, env.Error)
	}
	return env.Result, nil
}

func (c *rpcClient) specVersion(ctx context.Context) (string, error) {
	return rpcCall[string](ctx, c, "starknet_specVersion", nil)
}
