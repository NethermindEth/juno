// Package ugorji decodes CBOR with Ugorji.
package ugorji

import (
	"fmt"
	"sync"

	"github.com/ugorji/go/codec"
)

var (
	decoders = sync.Pool{New: func() any {
		return codec.NewDecoderBytes(nil, handle)
	}}
	handle = func() *codec.CborHandle {
		h := &codec.CborHandle{}
		h.TypeInfos = codec.NewTypeInfos([]string{"cbor"})
		// Fail on field names from older records rather than drop them, e.g. "GasPrice".
		h.ErrorIfNoField = true
		h.ValidateUnicode = true
		// Limit nesting to 32 levels; keep Ugorji's copying ownership.
		h.MaxDepth = 32
		return h
	}()
)

// Unmarshal decodes exactly one CBOR value. On error, out may be partially
// populated. Decoded values do not retain references to data.
func Unmarshal(data []byte, out any) error {
	d := decoders.Get().(*codec.Decoder)
	d.ResetBytes(data)
	err := d.Decode(out)
	consumed := d.NumBytesRead()
	d.ResetBytes(nil)
	decoders.Put(d)
	if err != nil {
		return err
	}
	if consumed != len(data) {
		return fmt.Errorf("cbor: %d trailing bytes", len(data)-consumed)
	}
	return nil
}
