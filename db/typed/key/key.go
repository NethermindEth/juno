package key

import "github.com/NethermindEth/juno/core/felt"

type Serializer[K any] interface {
	Marshal(K) []byte
}

var Empty = emptySerializer{}

var Bytes = bytesSerializer{}

var Uint64 = uint64Serializer{}

func Marshal[K marshaler[K]]() marshalSerializer[K] {
	return marshalSerializer[K]{}
}

var (
	Felt            = Marshal[*felt.Felt]()
	Address         = Marshal[*felt.Address]()
	ClassHash       = Marshal[*felt.ClassHash]()
	SierraClassHash = Marshal[*felt.SierraClassHash]()
	TransactionHash = Marshal[*felt.TransactionHash]()
)
