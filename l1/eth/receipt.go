package eth

import (
	"encoding/hex"
	"errors"
	"fmt"
)

// MaxDataBytesLen is the maximum allowed byte length for a DataBytes payload.
// In Ethereum, block gas limits cap total log data to ~3.75MB under an honest node.
// Capping to 4MB prevents unbounded memory allocation when connected to a dishonest/harmful RPC node.
const MaxDataBytesLen = 4 * 1024 * 1024 // 4 MB

// DataBytes is a []byte that JSON-decodes from a 0x-prefixed hex string,
// matching the Ethereum JSON-RPC "data" wire format. An empty payload ("0x") is allowed.
// Capped at [MaxDataBytesLen] to protect against unbounded memory allocation from dishonest clients.
type DataBytes []byte

func (h *DataBytes) UnmarshalJSON(input []byte) error {
	raw, err := unquoteHex(input)
	if err != nil {
		return err
	}
	if len(raw)%2 != 0 {
		return errors.New("odd length")
	}
	byteLen := len(raw) / 2
	if byteLen > MaxDataBytesLen {
		return fmt.Errorf("data length %d bytes exceeds maximum allowed limit (%d bytes)", byteLen, MaxDataBytesLen)
	}
	out := make([]byte, byteLen)
	if _, err := hex.Decode(out, raw); err != nil {
		return fmt.Errorf("invalid hex: %w", err)
	}
	*h = out
	return nil
}

// Log represents a contract log event as juno consumes it. Only the
// fields actually read today are declared; other fields in an eth_getLogs
// response are silently dropped during unmarshal.
type Log struct {
	Topics      []Hash    `json:"topics"`
	Data        DataBytes `json:"data"`
	BlockNumber HexU64    `json:"blockNumber"`
	Removed     bool      `json:"removed"`
}

// Receipt is the trimmed-down view of an Ethereum transaction receipt.
// Only Logs is read in juno. All other fields in the
// eth_getTransactionReceipt JSON response are ignored.
type Receipt struct {
	Logs []Log `json:"logs"`
}
