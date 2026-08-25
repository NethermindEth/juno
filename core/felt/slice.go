package felt

import "github.com/NethermindEth/juno/utils/cbor"

type Slice[F FeltLike] []F

var (
	_ cbor.SelfEncoder = Slice[Felt](nil)
	_ cbor.SelfDecoder = (*Slice[Felt])(nil)
)

func (s Slice[F]) MarshalCBOR() ([]byte, error) {
	return cbor.MarshalFeltSlice(s)
}

func (s *Slice[F]) UnmarshalCBOR(data []byte) error {
	return cbor.UnmarshalFeltSlice(data, (*[]F)(s))
}
