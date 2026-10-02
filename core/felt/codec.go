package felt

import (
	"errors"

	"github.com/ugorji/go/codec"
)

var _ codec.Selfer = (*Felt)(nil)

var errUgorjiFeltShape = errors.New("felt: unsupported CBOR shape for ugorji")

// CodecEncodeSelf adapts the existing CBOR hook to Ugorji.
func (z *Felt) CodecEncodeSelf(e *codec.Encoder) {
	data, err := z.MarshalCBOR()
	if err != nil {
		panic(err)
	}
	e.MustEncode(codec.Raw(data))
}

// CodecDecodeSelf reads a definite-length array of four unsigned limbs directly
// from the decoder; a null felt or limb decodes as zero. GenHelper is the API
// codecgen output uses and may change, so tests pin it for the version in go.mod.
func (z *Felt) CodecDecodeSelf(d *codec.Decoder) {
	gd, dd := codec.GenHelper().Decoder(d)
	if gd.DecReadArrayStart() != Limbs {
		panic(errUgorjiFeltShape)
	}
	var limbs Felt
	for i := range limbs {
		gd.DecReadArrayElem()
		limbs[i] = dd.DecodeUint64()
	}
	gd.DecReadArrayEnd()
	*z = limbs
}
