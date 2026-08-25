package felt

import "github.com/NethermindEth/juno/utils/cbor"

var (
	_ cbor.SelfEncoder = (*Felt)(nil)
	_ cbor.SelfDecoder = (*Felt)(nil)
)

// Fast, felt-specialized CBOR marshaling.
func (z *Felt) MarshalCBOR() ([]byte, error) {
	return cbor.MarshalFelt(z)
}

// Fast, felt-specialized CBOR unmarshaling.
// Falls back to the generic decoder on shape mismatch
func (z *Felt) UnmarshalCBOR(data []byte) error {
	return cbor.UnmarshalFelt(data, z)
}
