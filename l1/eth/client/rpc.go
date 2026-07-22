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

// ID is always uint64 on the way out; responses may carry number- or
// string-shaped ids (see parseResponseID).
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

// RPCError is the JSON-RPC error object from the remote. The methods layer maps specific
// (method, code) pairs into juno sentinels — e.g. -32000 for resource-missing replies.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// A well-formed response carries either Result or Error, never both. ID is kept as
// json.RawMessage to match both `42` and `"42"` shapes — the spec permits either.
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

// Accepts number ("42") and string ("\"42\"") shaped ids; rejects null (null
// id means the server couldn't determine which request it was answering).
func parseResponseID(raw json.RawMessage) (uint64, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, jsonNull) {
		return 0, errors.New("missing or null id")
	}
	if len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	n, err := strconv.ParseUint(string(trimmed), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse id %q: %w", raw, err)
	}
	return n, nil
}

// decodeSubID decodes the server-assigned subscription id from an
// eth_subscribe result. An empty id is rejected: it would register a sub
// under "" that no notification could ever match.
func decodeSubID(raw json.RawMessage) (string, error) {
	var subID string
	if err := json.Unmarshal(raw, &subID); err != nil {
		return "", fmt.Errorf("decoding subscription id: %w", err)
	}
	if subID == "" {
		return "", errors.New("empty subscription id")
	}
	return subID, nil
}
