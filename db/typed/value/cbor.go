package value

import "github.com/NethermindEth/juno/utils/cbor"

type cborSerializer[V any] struct{}

func (cborSerializer[V]) Marshal(value *V) ([]byte, error) {
	return cbor.Marshal(value)
}

func (cborSerializer[V]) Unmarshal(data []byte, value *V) error {
	return cbor.Unmarshal(data, value)
}
