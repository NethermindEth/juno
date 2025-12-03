package key

import "github.com/NethermindEth/juno/encoder"

type cborSerializer[K any] struct{}

func (cborSerializer[K]) Marshal(value K) []byte {
	data, err := encoder.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
