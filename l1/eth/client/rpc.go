// Package client speaks JSON-RPC 2.0 to an Ethereum execution-layer node
// over HTTP or WebSocket. It implements the small surface juno needs to
// follow the L1 head and serve starknet_getMessageStatus — it is not a
// general-purpose Ethereum RPC library.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
)

const jsonrpcVersion = "2.0"

// rpcRequest is the JSON-RPC 2.0 request envelope. Params is always an
// array; methods with no arguments serialise "params":[].
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
// response carries either Result or Error, never both.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
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
	if resp.ID != reqID {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrIDMismatch, resp.ID, reqID)
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}
