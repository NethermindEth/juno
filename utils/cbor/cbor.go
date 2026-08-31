// Package cbor wraps github.com/fxamacker/cbor/v2 so the rest of the node does not name it.
package cbor

import (
	"io"

	fxcbor "github.com/fxamacker/cbor/v2"
)

type (
	RawMessage         = fxcbor.RawMessage
	Marshaler          = fxcbor.Marshaler
	Unmarshaler        = fxcbor.Unmarshaler
	UnmarshalTypeError = fxcbor.UnmarshalTypeError
	Encoder            = fxcbor.Encoder
	Decoder            = fxcbor.Decoder
	EncMode            = fxcbor.EncMode
	DecMode            = fxcbor.DecMode
	EncOptions         = fxcbor.EncOptions
	DecOptions         = fxcbor.DecOptions
	TagSet             = fxcbor.TagSet
	TagOptions         = fxcbor.TagOptions
)

const (
	EncTagRequired = fxcbor.EncTagRequired
	DecTagRequired = fxcbor.DecTagRequired

	// ExtraDecErrorUnknownField makes a decoder fail on a wire key with no matching field.
	ExtraDecErrorUnknownField = fxcbor.ExtraDecErrorUnknownField
)

func NewTagSet() TagSet {
	return fxcbor.NewTagSet()
}

func CanonicalEncOptions() EncOptions {
	return fxcbor.CanonicalEncOptions()
}

func Marshal(v any) ([]byte, error) {
	return fxcbor.Marshal(v)
}

func Unmarshal(data []byte, v any) error {
	return fxcbor.Unmarshal(data, v)
}

func NewEncoder(w io.Writer) *Encoder {
	return fxcbor.NewEncoder(w)
}

func NewDecoder(r io.Reader) *Decoder {
	return fxcbor.NewDecoder(r)
}
