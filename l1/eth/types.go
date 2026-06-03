package eth

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

// HexU64 is a uint64 that JSON-decodes from a 0x-prefixed hex string,
// matching the Ethereum JSON-RPC "quantity" wire format. Used as a field
// type on structs that consume RPC responses (Header, Log).
type HexU64 uint64

func (h *HexU64) UnmarshalJSON(input []byte) error {
	raw, err := unquoteHex(input)
	if err != nil {
		return fmt.Errorf("eth: hex uint64: %w", err)
	}
	if len(raw) == 0 {
		return fmt.Errorf("eth: hex uint64 has no digits")
	}
	if len(raw) > 16 {
		return fmt.Errorf("eth: hex uint64 overflow (%d nibbles)", len(raw))
	}
	v, err := strconv.ParseUint(string(raw), 16, 64)
	if err != nil {
		return fmt.Errorf("eth: invalid hex uint64: %w", err)
	}
	*h = HexU64(v)
	return nil
}

// HexBytes is a []byte that JSON-decodes from a 0x-prefixed hex string,
// matching the Ethereum JSON-RPC "data" wire format. An empty payload
// ("0x") is allowed.
type HexBytes []byte

func (h *HexBytes) UnmarshalJSON(input []byte) error {
	raw, err := unquoteHex(input)
	if err != nil {
		return fmt.Errorf("eth: hex bytes: %w", err)
	}
	if len(raw)%2 != 0 {
		return fmt.Errorf("eth: hex bytes has odd length")
	}
	out := make([]byte, len(raw)/2)
	if _, err := hex.Decode(out, raw); err != nil {
		return fmt.Errorf("eth: invalid hex bytes: %w", err)
	}
	*h = out
	return nil
}

// unquoteHex strips the surrounding JSON quotes and the required 0x
// prefix, returning the hex digits. Accepts "0x" or "0X" prefix.
func unquoteHex(input []byte) ([]byte, error) {
	if len(input) < 2 || input[0] != '"' || input[len(input)-1] != '"' {
		return nil, fmt.Errorf("not a JSON string")
	}
	s := input[1 : len(input)-1]
	if len(s) < 2 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return nil, fmt.Errorf("missing 0x prefix")
	}
	return s[2:], nil
}

// Header is the trimmed-down view of an Ethereum block header that juno
// needs. Only Number is read in production (l1/eth_subscriber.go).
// The HexU64 field type lets eth_getBlockByNumber responses unmarshal
// directly without any per-struct codec boilerplate.
type Header struct {
	Number HexU64 `json:"number"`
}

// Subscription mirrors go-ethereum's event.Subscription surface as
// consumed by juno's l1 package: a channel signalling failure, and an
// Unsubscribe method to release resources.
type Subscription interface {
	Err() <-chan error
	Unsubscribe()
}

// Log represents a contract log event as juno consumes it. Only the
// fields actually read today are declared; other fields in an eth_getLogs
// response are silently dropped during unmarshal.
type Log struct {
	Topics      []Hash   `json:"topics"`
	Data        HexBytes `json:"data"`
	BlockNumber HexU64   `json:"blockNumber"`
	Removed     bool     `json:"removed"`
}

// Receipt is the trimmed-down view of an Ethereum transaction receipt.
// Only Logs is read in juno. All other fields in the
// eth_getTransactionReceipt JSON response are ignored.
type Receipt struct {
	Logs []*Log `json:"logs"`
}
