package eth

import (
	"encoding/hex"
	"fmt"
)

// Lengths of hashes and addresses in bytes.
const (
	// AddressLength is the expected length of the address.
	AddressLength = 20
	addressHexLen = 2 + 2*AddressLength // "0x" + 40 hex chars
)

// Address represents the 20 byte address of an Ethereum account.
type Address [AddressLength]byte

// BytesToAddress returns Address with value b.
// If b is larger than len(h), b will be cropped from the left.
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

// HexToAddress returns Address with byte values of s.
// If s is larger than len(h), s will be cropped from the left.
func HexToAddress(s string) Address { return BytesToAddress(fromHex(s)) }

// SetBytes sets the address to the value of b.
// If b is larger than len(a), b will be cropped from the left.
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
}

// Bytes gets the string representation of the underlying address.
func (a Address) Bytes() []byte { return a[:] }

// MarshalJSON returns the address as a JSON string (0x-prefixed lowercase hex),
// byte-for-byte identical to go-ethereum's common.Address.MarshalJSON.
func (a Address) MarshalJSON() ([]byte, error) {
	out := make([]byte, addressHexLen+2)
	out[0] = '"'
	out[1] = '0'
	out[2] = 'x'
	hex.Encode(out[3:], a[:])
	out[addressHexLen+1] = '"'
	return out, nil
}

// UnmarshalJSON parses an address from a JSON string. Matches go-ethereum's
// common.Address.UnmarshalJSON: requires a quoted "0x"-prefixed hex string of
// exactly AddressLength bytes; case-insensitive.
func (a *Address) UnmarshalJSON(input []byte) error {
	if len(input) < 2 || input[0] != '"' || input[len(input)-1] != '"' {
		return fmt.Errorf("eth: address must be a JSON string")
	}
	s := input[1 : len(input)-1]
	if len(s) < 2 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return fmt.Errorf("eth: address missing 0x prefix")
	}
	s = s[2:]
	if len(s) != 2*AddressLength {
		return fmt.Errorf("eth: invalid address length %d, want %d", len(s), 2*AddressLength)
	}
	if _, err := hex.Decode(a[:], s); err != nil {
		return fmt.Errorf("eth: invalid address hex: %w", err)
	}
	return nil
}
