package eth

import "bytes"

// HashLength is the expected length of the hash.
const HashLength = 32

// Hash represents the 32 byte Keccak256 hash of arbitrary data.
type Hash [HashLength]byte

// HashFromBytes returns Hash with value b.
// If b is larger than HashLength, b will be cropped from the left.
func HashFromBytes(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

// HashFromString returns Hash with byte values parsed from s. The 0x prefix
// is optional; if s is larger than 2*HashLength hex chars, it will be
// cropped from the left.
func HashFromString(s string) Hash { return HashFromBytes(fromHex(s)) }

// Cmp compares two hashes.
func (h Hash) Cmp(other Hash) int {
	return bytes.Compare(h[:], other[:])
}

// Bytes returns a copy of the underlying byte representation of the hash.
func (h Hash) Bytes() []byte { return h[:] }

// Hex converts a hash to a 0x-prefixed lowercase hex string.
func (h Hash) Hex() string { return string(encodeHex(h[:])) }

// SetBytes sets the hash to the value of b. If b is larger than HashLength,
// b will be cropped from the left; if shorter, left-padded with zeros.
// TODO: Once migrated, panic/error if b is larger than hash
func (h *Hash) SetBytes(b []byte) { setBytes(h[:], b) }

// MarshalText returns the hex representation of h (0x-prefixed lowercase).
// Mirrors go-ethereum's common.Hash.MarshalText.
func (h Hash) MarshalText() ([]byte, error) { return encodeHex(h[:]), nil }

// UnmarshalText parses a hash from its hex representation. Matches
// go-ethereum's common.Hash.UnmarshalText: requires "0x"-prefixed hex of
// exactly HashLength bytes; case-insensitive.
func (h *Hash) UnmarshalText(input []byte) error { return decodeHexText(h[:], input) }

// UnmarshalJSON parses a hash from a JSON string. Matches go-ethereum's
// common.Hash.UnmarshalJSON: requires a quoted "0x"-prefixed hex string of
// exactly HashLength bytes; case-insensitive.
func (h *Hash) UnmarshalJSON(input []byte) error { return decodeHexJSON(h[:], input) }
