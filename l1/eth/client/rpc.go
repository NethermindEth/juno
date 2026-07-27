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

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// rpcResponse keeps ID as json.RawMessage: the spec lets servers reply with
// number- or string-shaped ids (`42` or `"42"`).
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

// parseResponseID rejects a null id: the server couldn't tell which request
// it was answering, so there is no caller to route the reply to.
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
		return 0, fmt.Errorf("parsing id %q: %w", raw, err)
	}
	return n, nil
}

// decodeSubID rejects an empty id: it would register a sub under "" that no
// notification could ever match.
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
