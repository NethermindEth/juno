package value

import (
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
)

type cborSerializer[V any] struct{}

func (cborSerializer[V]) Marshal(value *V) ([]byte, error) {
	return cbor.Marshal(value)
}

func (cborSerializer[V]) Unmarshal(data []byte, value *V) error {
	return cbor.Unmarshal(data, value)
}

// fastSerializer writes with fxamacker and reads with the fastcbor package.
type fastSerializer[V any] struct{}

func (fastSerializer[V]) Marshal(value *V) ([]byte, error) {
	return cbor.Marshal(value)
}

func (fastSerializer[V]) Unmarshal(data []byte, value *V) error {
	return fastcbor.Unmarshal(data, value)
}
