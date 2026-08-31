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

const (
	maxRetries   = 3
	retryBackoff = 500 * time.Millisecond
)

type rpcClient struct {
	url    string
	client *http.Client
}

func newRPCClient(url string, maxConns int) *rpcClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = max(transport.MaxIdleConns, maxConns)
	transport.MaxIdleConnsPerHost = maxConns
	return &rpcClient{
		url:    url,
		client: &http.Client{Timeout: time.Minute, Transport: transport},
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

func (c *rpcClient) rpcCall[T any](ctx context.Context, method string, params any) (T, error) {
	var zero T
	reqBody, err := json.Marshal(jsonRPCRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return zero, err
	}

	var env rpcEnvelope[T]
	for attempt := 0; ; attempt++ {
		// Reset so a failed attempt's partial decode never leaks into the next.
		env = rpcEnvelope[T]{}
		err := post(ctx, c, method, reqBody, &env)
		if err == nil {
			break
		}
		if attempt == maxRetries {
			return zero, err
		}
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(retryBackoff << attempt):
		}
	}

	if env.Error != nil {
		return zero, fmt.Errorf("%s: %w", method, env.Error)
	}
	return env.Result, nil
}

func post(ctx context.Context, c *rpcClient, method string, body []byte, env any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"%s: unexpected status %s: %s",
			method,
			resp.Status,
			bytes.TrimSpace(respBody),
		)
	}

	if err := json.NewDecoder(resp.Body).Decode(env); err != nil {
		return fmt.Errorf("%s: decode response: %w", method, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *rpcClient) specVersion(ctx context.Context) (string, error) {
	return c.rpcCall[string](ctx, "starknet_specVersion", nil)
}

func (c *rpcClient) blockNumber(ctx context.Context) (uint64, error) {
	return c.rpcCall[uint64](ctx, "starknet_blockNumber", nil)
}

func (c *rpcClient) blockWithTxHashes(
	ctx context.Context,
	blockNumber uint64,
) (txHashesBlock, error) {
	return c.rpcCall[txHashesBlock](
		ctx,
		"starknet_getBlockWithTxHashes",
		blockIDParams{BlockID: blockNumberID{blockNumber}},
	)
}

func (c *rpcClient) txCountInBlock(ctx context.Context, blockNumber uint64) (uint64, error) {
	return c.rpcCall[uint64](
		ctx,
		"starknet_getBlockTransactionCount",
		blockIDParams{BlockID: blockNumberID{blockNumber}},
	)
}

func (c *rpcClient) stateUpdateAt(ctx context.Context, blockNumber uint64) (stateUpdate, error) {
	return c.rpcCall[stateUpdate](
		ctx,
		"starknet_getStateUpdate",
		blockIDParams{BlockID: blockNumberID{blockNumber}},
	)
}

func (c *rpcClient) classHashAt(
	ctx context.Context,
	blockNumber uint64,
	address string,
) (string, error) {
	return c.rpcCall[string](
		ctx,
		"starknet_getClassHashAt",
		contractAtBlockParams{BlockID: blockNumberID{blockNumber}, ContractAddress: address},
	)
}

func (c *rpcClient) classAt(
	ctx context.Context,
	blockNumber uint64,
	classHash string,
) (contractClass, error) {
	return c.rpcCall[contractClass](
		ctx,
		"starknet_getClass",
		classAtBlockParams{BlockID: blockNumberID{blockNumber}, ClassHash: classHash},
	)
}

func (c *rpcClient) blockWithReceipts(
	ctx context.Context,
	blockNumber uint64,
) (receiptsBlock, error) {
	return c.rpcCall[receiptsBlock](
		ctx,
		"starknet_getBlockWithReceipts",
		blockIDParams{BlockID: blockNumberID{blockNumber}},
	)
}
