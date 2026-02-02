package types

import (
	"github.com/fxamacker/cbor/v2"
	"github.com/holiman/uint256"
)

type L1Address U256

// Bytes returns the 32-byte big-endian representation.
func (a *L1Address) Bytes() [32]byte {
	return (*U256)(a).Bytes32()
}

// String returns hex representation with "0x" prefix.
func (a *L1Address) String() string {
	return (*U256)(a).String()
}

// Marshal returns the 32-byte big-endian representation as a slice.
func (a *L1Address) Marshal() []byte {
	b := (*U256)(a).Bytes32()
	return b[:]
}

// Unmarshal sets the value from big-endian bytes (left-padded if < 32 bytes).
func (a *L1Address) Unmarshal(e []byte) {
	if len(e) > 32 {
		e = e[len(e)-32:]
	}
	(*uint256.Int)((*U256)(a)).SetBytes(e)
}

// MarshalJSON encodes as a hex string with "0x" prefix.
func (a *L1Address) MarshalJSON() ([]byte, error) {
	return (*U256)(a).MarshalJSON()
}

// UnmarshalJSON decodes from hex string or decimal string.
func (a *L1Address) UnmarshalJSON(data []byte) error {
	return (*U256)(a).UnmarshalJSON(data)
}

// SetBytesCanonical accepts up to 32 bytes big-endian and left-pads to 32.
func (a *L1Address) SetBytesCanonical(data []byte) error {
	if len(data) > 32 {
		return errU256TooLarge
	}
	(*uint256.Int)((*U256)(a)).SetBytes(data)
	return nil
}

// MarshalCBOR marshals a L1Address to CBOR.
// For Ethereum addresses (first 12 bytes are zero), it marshals only the last 20 bytes
// to maintain compatibility with the old format. It first checks if this
// is an Ethereum address (first 12 bytes are zero) and if so, it marshals only the last 20 bytes.
// Otherwise, it marshals all 32 bytes.
func (a *L1Address) MarshalCBOR() ([]byte, error) {
	bytes := a.Bytes()

	isEthAddress := true
	for i := range 12 {
		if bytes[i] != 0 {
			isEthAddress = false
			break
		}
	}

	if isEthAddress {
		return cbor.Marshal(bytes[12:])
	}

	return cbor.Marshal(bytes[:])
}

// UnmarshalCBOR unmarshals a CBOR-encoded L1Address.
func (a *L1Address) UnmarshalCBOR(data []byte) error {
	var b []byte
	if err := cbor.Unmarshal(data, &b); err != nil {
		return err
	}

	(*uint256.Int)((*U256)(a)).SetBytes(b)
	return nil
}
