// Package ugorji decodes CBOR with Ugorji.
package ugorji

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
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
		h.ErrorIfNoField = true // Reject unknown fields.
		h.ValidateUnicode = true
		h.MaxDepth = 32
		h.MaxInitLen = 1 << 18 // Covers large felt slices.
		if err := h.SetInterfaceExt(reflect.TypeFor[big.Int](), bignumTag, bignum{}); err != nil {
			panic(err)
		}
		return h
	}()
)

// bignumTag identifies an unsigned CBOR bignum.
const bignumTag = 2

var errBignumShape = errors.New("ugorji: unsupported CBOR shape for big.Int")

// bignum decodes unsigned CBOR bignums into big.Int.
type bignum struct{}

// ConvertExt is required by [codec.InterfaceExt]; encoding is unsupported.
func (bignum) ConvertExt(any) any { panic("ugorji: encoding big.Int is not supported") }

// UpdateExt sets dst from the bignum's magnitude bytes.
func (bignum) UpdateExt(dst, src any) {
	magnitude, ok := src.([]byte)
	if !ok {
		panic(errBignumShape)
	}
	dst.(*big.Int).SetBytes(magnitude)
}

// Unmarshal decodes exactly one CBOR value from data into out.
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
