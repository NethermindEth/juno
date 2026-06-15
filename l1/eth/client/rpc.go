// Package client speaks JSON-RPC 2.0 to an Ethereum execution-layer node
// over WebSocket. It implements the small surface juno needs to follow
// the L1 head and serve starknet_getMessageStatus — it is not a
// general-purpose Ethereum RPC library. WebSocket-only because
// subscribe-based log delivery (eth_subscribe) requires a long-lived
// connection that HTTP doesn't provide; unary calls share the same conn.
package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

const jsonrpcVersion = "2.0"

// rpcRequest is the JSON-RPC 2.0 request envelope. Params is always an
// array; methods with no arguments serialise "params":[]. ID is always
// uint64 on the way out; we accept either number- or string-shaped ids
// on the response (see parseResponseID).
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

// RPCError is the JSON-RPC error object as returned by the remote endpoint.
// Code -32000 (server error) is the common umbrella every provider uses
// for resource-missing replies; the methods layer interprets specific
// (method, error) pairs into juno sentinels.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// rpcResponse is the JSON-RPC 2.0 response envelope. A well-formed
// response carries either Result or Error, never both. ID is kept as
// json.RawMessage so we can match both `42` and `"42"` shapes against
// our outgoing uint64 — the spec permits either, and providers diverge.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

func (e *RPCError) Error() string {
	if len(e.Data) > 0 {
		return fmt.Sprintf("json-rpc error %d: %s: %s", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("json-rpc error %d: %s", e.Code, e.Message)
}

// ErrIDMismatch indicates the response id did not match the request id —
// a protocol violation that means the remote replied to a different
// request than the one we sent.
var ErrIDMismatch = errors.New("response id mismatch")

// parseResponseID parses a JSON-RPC response id into uint64. The spec
// allows id to be number, string, or null; servers vary, so accept
// number ("42") and string ("\"42\"") shapes and reject everything
// else (null included — null id means the server couldn't determine
// which request it was answering).
func parseResponseID(raw json.RawMessage) (uint64, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, jsonNull) {
		return 0, errors.New("missing or null id")
	}
	// Strip a pair of surrounding quotes if the id is string-shaped.
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	n, err := strconv.ParseUint(string(trimmed), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse id %q: %w", raw, err)
	}
	return n, nil
}

// DecodeResponse parses a single JSON-RPC response body and returns the
// raw Result payload. A server-side RPCError is returned verbatim so
// callers can match on Code if needed; the methods layer is responsible
// for mapping resource-missing replies to eth.ErrNotFound (because a
// "not found" string can also come back from -32601 "method not found"
// and similar, where the meaning is unrelated).
func DecodeResponse(body []byte, reqID uint64) (json.RawMessage, error) {
	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	id, err := parseResponseID(resp.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIDMismatch, err)
	}
	if id != reqID {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrIDMismatch, id, reqID)
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}
