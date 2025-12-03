package value

import "github.com/NethermindEth/juno/encoder"

type cborSerializer[V any] struct{}

func (cborSerializer[V]) Marshal(value *V) ([]byte, error) {
	return encoder.Marshal(value)
}

func (cborSerializer[V]) Unmarshal(data []byte, value *V) error {
	return encoder.Unmarshal(data, value)
}
