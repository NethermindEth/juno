package key

import "github.com/NethermindEth/juno/core/felt"

type Serializer[K any] interface {
	~struct{}
	Marshal(K) []byte
}

var Empty = emptySerializer{}

var Bytes = BytesSerializer{}

var Uint8 = uint8Serializer{}

var Uint64 = uint64Serializer{}

func Cbor[K any]() cborSerializer[K] {
	return cborSerializer[K]{}
}

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
