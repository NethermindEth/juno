package eth

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
)

// setBytes copies b into dst, left-cropping if too long, left-padding with
// zeros if too short. Mirrors go-ethereum's common.Address/Hash.SetBytes.
func setBytes(dst, b []byte) {
	if len(b) > len(dst) {
		b = b[len(b)-len(dst):]
	}
	copy(dst[len(dst)-len(b):], b)
}

// encodeHex returns a 0x-prefixed lowercase hex encoding of b.
func encodeHex(b []byte) []byte {
	// 2 bytes for the "0x" prefix + 2 hex chars per byte.
	out := make([]byte, 2+2*len(b))
	out[0] = '0'
	out[1] = 'x'
	hex.Encode(out[2:], b)
	return out
}

// decodeHexText decodes a 0x-prefixed hex string into dst. dst's length is
// the expected byte count; a mismatched or malformed input is an error.
// Accepts both "0x" and "0X" prefixes; the hex body is case-insensitive.
func decodeHexText(dst, s []byte) error {
	if len(s) < 2 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return errors.New("missing 0x prefix")
	}
	s = s[2:]
	// 2 hex chars per byte.
	if len(s) != 2*len(dst) {
		return fmt.Errorf("invalid length %d, want %d", len(s), 2*len(dst))
	}
	if _, err := hex.Decode(dst, s); err != nil {
		return fmt.Errorf("invalid hex: %w", err)
	}
	return nil
}

// decodeHexJSON decodes a JSON-quoted, 0x-prefixed hex string into dst.
func decodeHexJSON(dst, input []byte) error {
	if len(input) < 2 || input[0] != '"' || input[len(input)-1] != '"' {
		return errors.New("must be a JSON string")
	}
	return decodeHexText(dst, input[1:len(input)-1])
}

// unquoteHex strips the surrounding JSON quotes and the required 0x prefix,
// returning the hex digits. Accepts "0x" or "0X" prefix.
func unquoteHex(input []byte) ([]byte, error) {
	if len(input) < 2 || input[0] != '"' || input[len(input)-1] != '"' {
		return nil, errors.New("not a JSON string")
	}
	s := input[1 : len(input)-1]
	if len(s) < 2 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return nil, errors.New("missing 0x prefix")
	}
	return s[2:], nil
}

// fromHex returns the bytes represented by the hexadecimal string s. The 0x
// prefix is optional; an odd-length input is left-padded with a leading zero;
// invalid hex characters cause a partial/empty result (no error).
func fromHex(s string) []byte {
	if has0xPrefix(s) {
		s = s[2:]
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	b, _ := hex.DecodeString(s)
	return b
}

// has0xPrefix reports whether s begins with "0x" or "0X".
func has0xPrefix(s string) bool {
	return len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X')
}

// HexU64 is a uint64 that JSON-decodes from a 0x-prefixed hex string,
// matching the Ethereum JSON-RPC "quantity" wire format. Used as a field
// type on structs that consume RPC responses (Header, Log).
type HexU64 uint64

func (h *HexU64) UnmarshalJSON(input []byte) error {
	raw, err := unquoteHex(input)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return errors.New("no digits")
	}
	// JSON-RPC "quantity" values must be minimally encoded: "0x0" for zero,
	// otherwise no leading zeros. Mirrors go-ethereum's hexutil behavior.
	if len(raw) > 1 && raw[0] == '0' {
		return errors.New("leading zero")
	}
	if len(raw) > 16 {
		return fmt.Errorf("overflow (%d nibbles)", len(raw))
	}
	v, err := strconv.ParseUint(string(raw), 16, 64)
	if err != nil {
		return fmt.Errorf("invalid hex: %w", err)
	}
	*h = HexU64(v)
	return nil
}
