package felt

import (
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/encoder/cbor"
)

type Slice[F FeltLike] []F

var (
	_ encoder.SelfEncoder = Slice[Felt](nil)
	_ encoder.SelfDecoder = (*Slice[Felt])(nil)
)

func (s Slice[F]) MarshalCBOR() ([]byte, error) {
	return cbor.MarshalFeltSlice(s), nil
}

func (s *Slice[F]) UnmarshalCBOR(data []byte) error {
	if cbor.UnmarshalFeltSlice(data, (*[]F)(s)) {
		return nil
	}

	var raw []F
	if err := encoder.Unmarshal(data, &raw); err != nil {
		return err
	}

	*s = raw
	return nil
}
