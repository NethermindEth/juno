package value

import "github.com/NethermindEth/juno/utils/cbor"

type binaryMarshaler[V any] interface {
	*V
	MarshalBinary() ([]byte, error)
	UnmarshalBinary([]byte) error
}

type binarySerializer[V any, P binaryMarshaler[V]] struct{}

func (binarySerializer[V, P]) Marshal(value *V) ([]byte, error) {
	return P(value).MarshalBinary()
}

func (binarySerializer[V, P]) Unmarshal(data []byte, value *V) error {
	return P(value).UnmarshalBinary(data)
}

type binaryCborSerializer[V any, P binaryMarshaler[V]] struct{}

func (binaryCborSerializer[V, P]) Marshal(value *V) ([]byte, error) {
	return cbor.Marshal(value)
}

func (binaryCborSerializer[V, P]) Unmarshal(data []byte, value *V) error {
	return cbor.Unmarshal(data, value)
}
