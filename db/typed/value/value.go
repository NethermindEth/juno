package value

import "github.com/NethermindEth/juno/core/felt"

type Serializer[V any] interface {
	~struct{}
	Marshal(*V) ([]byte, error)
	Unmarshal([]byte, *V) error
}

var Uint64 = uint64Serializer{}

var (
	Felt      = feltBytesSerializer[*felt.Felt]{}
	Hash      = feltBytesSerializer[*felt.Hash]{}
	ClassHash = feltBytesSerializer[*felt.ClassHash]{}
)

var Bytes = BytesSerializer{}

func Binary[V any, P binaryMarshaler[V]]() binarySerializer[V, P] {
	return binarySerializer[V, P]{}
}

func Cbor[V any]() cborSerializer[V] {
	return cborSerializer[V]{}
}

// Fast is Cbor with the value read by the fastcbor package.
func Fast[V any]() fastSerializer[V] {
	return fastSerializer[V]{}
}
