package felt

import (
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/encoder/cbor"
)

var (
	_ encoder.SelfEncoder = (*Felt)(nil)
	_ encoder.SelfDecoder = (*Felt)(nil)
)

// Fast, felt-specialized CBOR marshaling.
func (z *Felt) MarshalCBOR() ([]byte, error) {
	return cbor.MarshalFelt(z), nil
}

// Fast, felt-specialized CBOR unmarshaling.
// Falls back to the generic decoder on shape mismatch
func (z *Felt) UnmarshalCBOR(data []byte) error {
	if cbor.UnmarshalFelt(data, z) {
		return nil
	}

	// An array has no unmarshal hook, so the engine cannot recurse in here.
	// A CBOR null decodes as a no-op, so raw has to carry the current value in.
	raw := [Limbs]uint64(*z)
	if err := encoder.Unmarshal(data, &raw); err != nil {
		return err
	}

	*z = Felt(raw)
	return nil
}
