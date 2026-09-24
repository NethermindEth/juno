package key

import "github.com/NethermindEth/juno/utils/cbor/v1"

type cborSerializer[K any] struct{}

func (cborSerializer[K]) Marshal(value K) []byte {
	data, err := cbor.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
