package types

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/holiman/uint256"
)

// U256 is a 256-bit unsigned integer (wraps holiman/uint256.Int).
type U256 uint256.Int

var (
	errU256TooLarge = errors.New("value exceeds 32 bytes")
	errU256BadHex   = errors.New("invalid hex string")
)

// Bytes32 returns the 32-byte big-endian representation.
func (u *U256) Bytes32() [32]byte {
	return (*uint256.Int)(u).Bytes32()
}

// Equal reports whether u and other represent the same value.
func (u *U256) Equal(other *U256) bool {
	if other == nil {
		return false
	}
	return (*uint256.Int)(u).Eq((*uint256.Int)(other))
}

// String returns hex representation with "0x" prefix.
func (u *U256) String() string {
	return (*uint256.Int)(u).Hex()
}

// Marshal returns the 32-byte big-endian representation as a slice.
func (u *U256) Marshal() []byte {
	b := u.Bytes32()
	return b[:]
}

// Unmarshal sets the value from big-endian bytes (right-truncated if > 32 bytes).
func (u *U256) Unmarshal(e []byte) {
	if len(e) > 32 {
		e = e[len(e)-32:]
	}
	(*uint256.Int)(u).SetBytes(e)
}

// MarshalJSON encodes as a hex string with "0x" prefix.
func (u *U256) MarshalJSON() ([]byte, error) {
	hexStr := (*uint256.Int)(u).Hex()
	return json.Marshal(hexStr)
}

// UnmarshalJSON decodes from hex string or decimal string.
func (u *U256) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	v, err := u256FromString(s)
	if err != nil {
		return err
	}
	*u = U256(*v)
	return nil
}

// MarshalCBOR marshals the 32-byte value to CBOR.
func (u *U256) MarshalCBOR() ([]byte, error) {
	return cbor.Marshal(u.Marshal())
}

// UnmarshalCBOR unmarshals a CBOR-encoded byte slice into u.
func (u *U256) UnmarshalCBOR(data []byte) error {
	var b []byte
	if err := cbor.Unmarshal(data, &b); err != nil {
		return err
	}
	u.Unmarshal(b)
	return nil
}

// SetBytesCanonical accepts up to 32 bytes big-endian and sets the value.
func (u *U256) SetBytesCanonical(data []byte) error {
	if len(data) > 32 {
		return errU256TooLarge
	}
	(*uint256.Int)(u).SetBytes(data)
	return nil
}

// SetBytes32 sets the value from exactly 32 big-endian bytes.
func (u *U256) SetBytes32(b [32]byte) error {
	(*uint256.Int)(u).SetBytes(b[:])
	return nil
}

func u256FromString(value string) (*U256, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errU256BadHex
	}
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		return u256FromHex(value[2:])
	}
	return u256FromDecimal(value)
}

func u256FromHex(hexPart string) (*U256, error) {
	hexPart = strings.TrimLeft(hexPart, "0")
	if hexPart == "" {
		hexPart = "0"
	}
	val, err := uint256.FromHex("0x" + hexPart)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errU256BadHex, err)
	}
	return (*U256)(val), nil
}

func u256FromDecimal(s string) (*U256, error) {
	val, err := uint256.FromDecimal(s)
	if err != nil {
		if bi, ok := new(big.Int).SetString(s, 10); ok && bi.BitLen() > 256 {
			return nil, errU256TooLarge
		}
		return nil, fmt.Errorf("%w: %v", errU256BadHex, err)
	}
	return (*U256)(val), nil
}

// RandomU256 returns a random U256 value.
func RandomU256() (*U256, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, err
	}
	u := new(U256)
	(*uint256.Int)(u).SetBytes(b[:])
	return u, nil
}
