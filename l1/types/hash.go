package types

import "github.com/holiman/uint256"

type L1Hash U256

// Bytes returns the 32-byte big-endian representation.
func (h *L1Hash) Bytes() [32]byte {
	return (*U256)(h).Bytes32()
}

// String returns hex representation with "0x" prefix.
func (h *L1Hash) String() string {
	return (*U256)(h).String()
}

// Marshal returns the 32-byte big-endian representation as a slice.
func (h *L1Hash) Marshal() []byte {
	b := (*U256)(h).Bytes32()
	return b[:]
}

// Unmarshal sets the value from big-endian bytes (right-truncated if > 32 bytes).
func (h *L1Hash) Unmarshal(e []byte) {
	if len(e) > 32 {
		e = e[len(e)-32:]
	}
	(*uint256.Int)((*U256)(h)).SetBytes(e)
}

// MarshalJSON encodes as a hex string with "0x" prefix.
func (h *L1Hash) MarshalJSON() ([]byte, error) {
	return (*U256)(h).MarshalJSON()
}

// UnmarshalJSON decodes from hex string or decimal string.
func (h *L1Hash) UnmarshalJSON(data []byte) error {
	return (*U256)(h).UnmarshalJSON(data)
}

// MarshalCBOR marshals the 32-byte value to CBOR.
func (h *L1Hash) MarshalCBOR() ([]byte, error) {
	u := (*U256)(h)
	return u.MarshalCBOR()
}

// UnmarshalCBOR unmarshals a CBOR-encoded 32-byte value.
func (h *L1Hash) UnmarshalCBOR(data []byte) error {
	u := (*U256)(h)
	return u.UnmarshalCBOR(data)
}

// SetBytesCanonical accepts up to 32 bytes big-endian and sets the value.
func (h *L1Hash) SetBytesCanonical(data []byte) error {
	if len(data) > 32 {
		return errU256TooLarge
	}
	(*uint256.Int)((*U256)(h)).SetBytes(data)
	return nil
}
