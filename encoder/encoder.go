package encoder

import (
	"io"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

// UnmarshalTypeError is a wire item that does not fit the Go type, as opposed to malformed input.
type UnmarshalTypeError = cbor.UnmarshalTypeError

var (
	ts = cbor.NewTagSet()
	// https://www.iana.org/assignments/cbor-tags/cbor-tags.xhtml
	// 65536-15309735 	Unassigned
	tagNum     uint64 = 65536
	encMode           = newEncMode()
	decMode           = newDecMode()
	strictMode        = newStrictMode()
)

func newEncMode() cbor.EncMode {
	mode, err := cbor.CanonicalEncOptions().EncModeWithTags(ts)
	if err != nil {
		panic(err)
	}
	return mode
}

func newDecMode() cbor.DecMode {
	mode, err := cbor.DecOptions{
		MaxArrayElements: 10485760, // Set to a reasonably high value, 10MiB
	}.DecModeWithTags(ts)
	if err != nil {
		panic(err)
	}
	return mode
}

func newStrictMode() cbor.DecMode {
	mode, err := cbor.DecOptions{
		ExtraReturnErrors: cbor.ExtraDecErrorUnknownField,
	}.DecMode()
	if err != nil {
		panic(err)
	}
	return mode
}

// RegisterType gives a unique CBOR tag to a type.
// Only call this from encoder/registry's init().
func RegisterType(rType reflect.Type) error {
	if err := ts.Add(
		cbor.TagOptions{EncTag: cbor.EncTagRequired, DecTag: cbor.DecTagRequired},
		rType,
		tagNum,
	); err != nil {
		return err
	}
	encMode, decMode = newEncMode(), newDecMode()
	tagNum++
	return nil
}

// Marshal returns encoding of param v
func Marshal(v any) ([]byte, error) {
	return encMode.Marshal(v)
}

// Unmarshal decodes param v from []byte b
func Unmarshal(b []byte, v any) error {
	return decMode.Unmarshal(b, v)
}

// UnmarshalFirst decodes the first CBOR data item into param v and returns the remaining bytes
func UnmarshalFirst(b []byte, v any) ([]byte, error) {
	return decMode.UnmarshalFirst(b, v)
}

// UnmarshalStrict decodes without type tags and fails on a wire key with no matching field.
// Only projection drift tests need it.
func UnmarshalStrict(b []byte, v any) error {
	return strictMode.Unmarshal(b, v)
}

type Encoder interface {
	Encode(v any) error
}

// NewEncoder returns a new encoder that writes to w
func NewEncoder(w io.Writer) Encoder {
	return encMode.NewEncoder(w)
}
