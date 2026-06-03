package eth

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

const (
	// HashLength is the expected length of the hash.
	HashLength = 32
	hashHexLen = 2 + 2*HashLength // "0x" + 64 hex chars
)

// Hash represents the 32 byte Keccak256 hash of arbitrary data.
type Hash [HashLength]byte

// BytesToHash sets b to hash.
// If b is larger than len(h), b will be cropped from the left.
func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

// HexToHash sets byte representation of s to hash.
// If b is larger than len(h), b will be cropped from the left.
func HexToHash(s string) Hash { return BytesToHash(fromHex(s)) }

// Cmp compares two hashes.
func (h Hash) Cmp(other Hash) int {
	return bytes.Compare(h[:], other[:])
}

// Bytes gets the byte representation of the underlying hash.
func (h Hash) Bytes() []byte { return h[:] }

// Hex converts a hash to a 0x-prefixed lowercase hex string.
func (h Hash) Hex() string {
	var buf [hashHexLen]byte
	buf[0] = '0'
	buf[1] = 'x'
	hex.Encode(buf[2:], h[:])
	return string(buf[:])
}

// SetBytes sets the hash to the value of b.
// If b is larger than len(h), b will be cropped from the left.
func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}

	copy(h[HashLength-len(b):], b)
}

// MarshalJSON returns the hash as a JSON string (0x-prefixed lowercase hex),
// byte-for-byte identical to go-ethereum's common.Hash.MarshalJSON.
func (h Hash) MarshalJSON() ([]byte, error) {
	out := make([]byte, hashHexLen+2)
	out[0] = '"'
	out[1] = '0'
	out[2] = 'x'
	hex.Encode(out[3:], h[:])
	out[hashHexLen+1] = '"'
	return out, nil
}

// UnmarshalJSON parses a hash from a JSON string. Matches go-ethereum's
// common.Hash.UnmarshalJSON: requires a quoted "0x"-prefixed hex string of
// exactly HashLength bytes; case-insensitive.
func (h *Hash) UnmarshalJSON(input []byte) error {
	if len(input) < 2 || input[0] != '"' || input[len(input)-1] != '"' {
		return fmt.Errorf("eth: hash must be a JSON string")
	}
	return h.decodeHex(input[1 : len(input)-1])
}

func (h *Hash) decodeHex(s []byte) error {
	if len(s) < 2 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return fmt.Errorf("eth: hash missing 0x prefix")
	}
	s = s[2:]
	if len(s) != 2*HashLength {
		return fmt.Errorf("eth: invalid hash length %d, want %d", len(s), 2*HashLength)
	}
	if _, err := hex.Decode(h[:], s); err != nil {
		return fmt.Errorf("eth: invalid hash hex: %w", err)
	}
	return nil
}

// fromHex returns the bytes represented by the hexadecimal string s. The 0x
// prefix is optional; an odd-length input is left-padded with a leading zero;
// invalid hex characters cause a partial/empty result (no error). Mirrors
// go-ethereum's common.FromHex behavior, which HexToAddress / HexToHash rely on.
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
