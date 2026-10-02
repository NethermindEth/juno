package core

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/ugorji/go/codec"
)

var _ codec.Selfer = (*Header)(nil)

// headerFields has Header's fields without its codec methods.
type headerFields Header

// storedHeader also reads keys older databases hold: "GasPrice" and
// "GasPriceSTRK" before #2335, and "ExtraData" before #1498.
type storedHeader struct {
	headerFields
	GasPrice     *felt.Felt
	GasPriceSTRK *felt.Felt
	ExtraData    *felt.Felt
}

// CodecEncodeSelf writes the current header keys.
func (h *Header) CodecEncodeSelf(e *codec.Encoder) {
	e.MustEncode((*headerFields)(h))
}

// CodecDecodeSelf reads current and legacy header keys.
func (h *Header) CodecDecodeSelf(d *codec.Decoder) {
	s := storedHeader{headerFields: headerFields(*h)}
	d.MustDecode(&s)
	*h = Header(s.headerFields)
	if s.GasPrice != nil {
		h.L1GasPriceETH = s.GasPrice
	}
	if s.GasPriceSTRK != nil {
		h.L1GasPriceSTRK = s.GasPriceSTRK
	}
}
