package encoder

import "errors"

// cborNull is what the generic encoder emits for a nil slice.
const cborNull = 0xf6

// RawMessage is an item that is already CBOR encoded.
// Do not tag a field of this type `,omitempty`. An empty one is written as null, not dropped.
type RawMessage []byte

var (
	_ SelfEncoder = RawMessage(nil)
	_ SelfDecoder = (*RawMessage)(nil)
)

// Returns the same bytes
func (m RawMessage) MarshalCBOR() ([]byte, error) {
	if len(m) == 0 {
		return []byte{cborNull}, nil
	}
	return m, nil
}

// Creates a copy of the bytes
func (m *RawMessage) UnmarshalCBOR(data []byte) error {
	if m == nil {
		return errors.New("encoder.RawMessage: UnmarshalCBOR on nil pointer")
	}
	*m = append((*m)[:0], data...)
	return nil
}
